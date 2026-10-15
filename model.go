package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const modelBaseURL = "https://huggingface.co/csukuangfj/sherpa-onnx-nemo-transducer-punct-giga-am-v3-russian-2025-12-16/resolve/a6039be7cee829a9044a69ac0ebaf1c191217c97/"

type modelFile struct {
	name   string
	size   int64
	sha256 string
}

var gigaAMFiles = []modelFile{
	{"encoder.int8.onnx", 224570820, "369f35a71bf288d3b8e0391fabd8dba5f2314088d440bca474056b7b4b6e66bf"},
	{"decoder.onnx", 4600132, "38fc7475443ea2a26f63211ca350f73ac50fff824ab7a3876ee2bd610c53bbc4"},
	{"joiner.onnx", 2712896, "602ff7017a93311aad34df1437c8d7f49911353c13d6eae7a6ee7b041339465c"},
	{"tokens.txt", 13354, "39abae20e692998290c574e606f11a9edef2902a1995463fcff63d1490cf22b7"},
}

func defaultModelDir() string {
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "voicetype", "gigaam-v3-ru")
}

func maracuyaModelDir() string {
	return filepath.Join(os.Getenv("HOME"), ".maracuya", "service", "voice", "models", "gigaam-v3-ru")
}

func modelReady(dir string, files []modelFile) (bool, error) {
	for _, file := range files {
		valid, err := validModelFile(filepath.Join(dir, file.name), file)
		if err != nil {
			return false, fmt.Errorf("проверить файл %s: %w", file.name, err)
		}
		if !valid {
			return false, nil
		}
	}
	return true, nil
}

func ensureModel(ctx context.Context, dir string, client *http.Client, baseURL string, files []modelFile) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("создать каталог модели %s: %w", dir, err)
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("загрузка модели отменена: %w", err)
		}
		path := filepath.Join(dir, file.name)
		valid, err := validModelFile(path, file)
		if err != nil {
			return fmt.Errorf("проверить файл модели %s: %w", path, err)
		}
		if valid {
			continue
		}
		if err := downloadModelFile(ctx, client, baseURL+file.name, path, file); err != nil {
			return fmt.Errorf("скачать файл модели %s: %w", file.name, err)
		}
	}
	return nil
}

func validModelFile(path string, file modelFile) (bool, error) {
	input, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Size() != file.size {
		return false, nil
	}
	if file.sha256 == "" {
		return true, nil
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, input); err != nil {
		return false, err
	}
	return hex.EncodeToString(hash.Sum(nil)) == file.sha256, nil
}

func downloadModelFile(ctx context.Context, client *http.Client, url, destination string, file modelFile) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("создать запрос: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("запросить модель: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("сервер вернул http %d", response.StatusCode)
	}
	if response.ContentLength > 0 && response.ContentLength != file.size {
		return fmt.Errorf("неверный размер ответа: %d", response.ContentLength)
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".model-*")
	if err != nil {
		return fmt.Errorf("создать временный файл: %w", err)
	}
	defer os.Remove(temporary.Name())
	defer temporary.Close()
	hash := sha256.New()
	count, err := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(response.Body, file.size+1))
	if err != nil {
		return fmt.Errorf("прочитать ответ: %w", err)
	}
	if count != file.size {
		return fmt.Errorf("неверный размер файла: %d вместо %d", count, file.size)
	}
	if file.sha256 != "" && !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), file.sha256) {
		return fmt.Errorf("контрольная сумма не совпала")
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("сохранить временный файл: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("закрыть временный файл: %w", err)
	}
	if err := os.Rename(temporary.Name(), destination); err != nil {
		return fmt.Errorf("установить файл модели: %w", err)
	}
	return nil
}

func modelHTTPClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Minute}
}
