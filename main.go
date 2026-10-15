package main

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

const sampleRate = 16000
const samplesPerChunk = 1600

type recognizer interface {
	Accept([]float32) (string, bool, error)
	Finish() (string, error)
	Close()
}

func main() {
	runtime.LockOSThread()
	args := os.Args[1:]
	state := newDictationState(nil)
	if slices.Contains(args, "-stdout") || slices.Contains(args, "--stdout") || slices.Contains(args, "-stdout=true") {
		if err := run(args, os.Stdout, os.Stderr, state); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	status, cleanup := newMenuStatus(state)
	defer cleanup()
	state.report = status.update
	go func() {
		if err := run(args, os.Stdout, os.Stderr, state); err != nil {
			fmt.Fprintln(os.Stderr, err)
			state.unavailable(err)
			return
		}
		stopMenuStatus()
	}()
	status.run()
}

func run(args []string, stdout, stderr io.Writer, state *dictationState) error {
	flags := flag.NewFlagSet("voicetype", flag.ContinueOnError)
	flags.SetOutput(stderr)
	modelDir := flags.String("model-dir", defaultModelDir(), "каталог модели GigaAM v3")
	library := flags.String("library", "", "путь к libsherpa-onnx-c-api.dylib")
	device := flags.String("device", ":default", "устройство ввода ffmpeg")
	threads := flags.Int("threads", 2, "число потоков распознавания")
	stdoutOnly := flags.Bool("stdout", false, "печатать текст в терминал вместо вставки")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("разобрать аргументы: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("неожиданные аргументы: %s", strings.Join(flags.Args(), " "))
	}
	if *threads < 1 {
		return errors.New("threads должен быть положительным")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("найти ffmpeg: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := slog.New(slog.NewTextHandler(stderr, nil))
	if !*stdoutOnly {
		logger.Info("доступ к управлению компьютером", "granted", accessibilityTrusted())
	}
	logger.Info("проверка модели GigaAM v3", "dir", *modelDir)
	if err := ensureModel(ctx, *modelDir, modelHTTPClient(), modelBaseURL, gigaAMFiles); err != nil {
		return err
	}
	paths, err := modelPaths(*modelDir)
	if err != nil {
		return err
	}
	libPath := *library
	if libPath == "" {
		libPath, err = defaultLibraryPath()
		if err != nil {
			return err
		}
	}
	decoder, err := newGigaAM(libPath, paths, *threads)
	if err != nil {
		return err
	}
	engine := newPhraseRecognizer(decoder)
	defer engine.Close()
	logger.Info("модель GigaAM v3 готова; текст вставляется после паузы", "dir", *modelDir)

	state.setReady()
	logger.Info("ожидание команды «Енот текст»; остановка — «Енот стоп»", "device", *device)
	deliver := func(ctx context.Context, text string) error {
		payload, event := state.accept(text)
		switch event {
		case gateStarted:
			logger.Info("диктовка включена")
		case gateStopped:
			logger.Info("диктовка остановлена")
		}
		payload = strings.TrimSpace(payload)
		if payload == "" {
			return nil
		}
		if *stdoutOnly {
			if _, err := fmt.Fprintln(stdout, payload); err != nil {
				return fmt.Errorf("вывести текст: %w", err)
			}
			return nil
		}
		if err := pasteText(ctx, payload); err != nil {
			state.fail(err)
			logger.Error("не удалось вставить текст; он остался в буфере обмена", "error", err)
		} else {
			state.pasted()
			logger.Info("команда вставки отправлена", "characters", utf8.RuneCountInString(payload))
		}
		return nil
	}
	return listen(ctx, *device, engine, deliver)
}

func listen(ctx context.Context, device string, engine recognizer, deliver func(context.Context, string) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-nostdin", "-loglevel", "error", "-f", "avfoundation", "-i", device, "-ar", "16000", "-ac", "1", "-f", "s16le", "-")
	var ffmpegErrors strings.Builder
	cmd.Stderr = &ffmpegErrors
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("открыть поток микрофона: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("запустить ffmpeg: %w", err)
	}

	chunks := make(chan []float32, 600)
	done := make(chan struct{})
	var captureErr error
	go func() {
		defer close(done)
		captureErr = readMicrophone(ctx, pipe, chunks)
		if captureErr != nil {
			pipe.Close()
		}
		close(chunks)
		waitErr := cmd.Wait()
		if captureErr == nil && ctx.Err() == nil && waitErr != nil {
			captureErr = fmt.Errorf("ffmpeg завершился: %w: %s", waitErr, strings.TrimSpace(ffmpegErrors.String()))
		}
	}()
	defer func() {
		cancel()
		pipe.Close()
		<-done
	}()
	for samples := range chunks {
		if ctx.Err() != nil {
			break
		}
		text, ready, err := engine.Accept(samples)
		if err != nil {
			return fmt.Errorf("распознать звук: %w", err)
		}
		if ready {
			if err := deliver(ctx, text); err != nil {
				return fmt.Errorf("вставить распознанный текст: %w", err)
			}
		}
	}
	if ctx.Err() == nil {
		<-done
		if captureErr != nil {
			return captureErr
		}
	}
	text, err := engine.Finish()
	if err != nil {
		return fmt.Errorf("завершить распознавание: %w", err)
	}
	if text != "" {
		finishCtx := ctx
		if ctx.Err() != nil {
			var cancel context.CancelFunc
			finishCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
		}
		if err := deliver(finishCtx, text); err != nil {
			return fmt.Errorf("вставить последнюю фразу: %w", err)
		}
	}
	return nil
}

// readMicrophone keeps capturing while the decoder processes a previous phrase.
func readMicrophone(ctx context.Context, input io.Reader, chunks chan<- []float32) error {
	buf := make([]byte, samplesPerChunk*2)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := io.ReadFull(input, buf)
		if n > 0 {
			samples := pcm16ToFloat32(buf[:n])
			select {
			case <-ctx.Done():
				return ctx.Err()
			case chunks <- samples:
			default:
				return errors.New("распознавание отстаёт более чем на 60 секунд")
			}
		}
		if readErr != nil {
			if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
				return nil
			}
			return fmt.Errorf("прочитать звук микрофона: %w", readErr)
		}
	}
}

func pcm16ToFloat32(pcm []byte) []float32 {
	samples := make([]float32, len(pcm)/2)
	for i := range samples {
		samples[i] = float32(int16(binary.LittleEndian.Uint16(pcm[i*2:]))) / 32768
	}
	return samples
}

type modelFiles struct {
	encoder string
	decoder string
	joiner  string
	tokens  string
}

func modelPaths(dir string) (modelFiles, error) {
	find := func(names ...string) (string, error) {
		for _, name := range names {
			file := filepath.Join(dir, name)
			if info, err := os.Stat(file); err == nil && info.Mode().IsRegular() {
				return file, nil
			}
		}
		return "", fmt.Errorf("нет файла модели в %s: %s", dir, strings.Join(names, " или "))
	}
	var files modelFiles
	var err error
	if files.encoder, err = find("encoder.int8.onnx", "encoder.onnx"); err != nil {
		return files, err
	}
	if files.decoder, err = find("decoder.int8.onnx", "decoder.onnx"); err != nil {
		return files, err
	}
	if files.joiner, err = find("joiner.int8.onnx", "joiner.onnx"); err != nil {
		return files, err
	}
	if files.tokens, err = find("tokens.txt"); err != nil {
		return files, err
	}
	return files, nil
}

func defaultLibraryPath() (string, error) {
	runtimeDir := filepath.Join(os.Getenv("HOME"), ".maracuya", "service", "voice", "runtime")
	versions, err := os.ReadDir(runtimeDir)
	if err != nil {
		return "", fmt.Errorf("прочитать каталог движка %s: %w", runtimeDir, err)
	}
	for i := len(versions) - 1; i >= 0; i-- {
		if !versions[i].IsDir() {
			continue
		}
		candidate := filepath.Join(runtimeDir, versions[i].Name(), "node_modules", "sherpa-onnx-darwin-arm64", "libsherpa-onnx-c-api.dylib")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("не найдена библиотека sherpa-onnx в %s", runtimeDir)
}
