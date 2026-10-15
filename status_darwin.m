#import <Cocoa/Cocoa.h>
#include <stdint.h>

extern void toggleDictation(uintptr_t handle);

@interface VoiceStatus : NSObject {
 NSStatusItem *item;
 NSMenuItem *stateItem;
 NSMenuItem *toggleItem;
 NSMenuItem *detailItem;
 uintptr_t handle;
}
- (id)initWithHandle:(uintptr_t)value;
- (void)updateActive:(BOOL)active ready:(BOOL)ready problem:(NSString *)problem heard:(NSString *)heard;
@end

@implementation VoiceStatus
- (id)initWithHandle:(uintptr_t)value {
 self = [super init];
 if (self) {
  handle = value;
  item = [[[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength] retain];
  item.button.image = [NSImage imageWithSystemSymbolName:@"mic" accessibilityDescription:@"Голосовой ввод Енот"];
  item.button.imagePosition = NSImageLeft;
  NSMenu *menu = [[[NSMenu alloc] init] autorelease];
  stateItem = [menu addItemWithTitle:@"Загрузка GigaAM…" action:nil keyEquivalent:@""];
  [menu addItem:[NSMenuItem separatorItem]];
  toggleItem = [menu addItemWithTitle:@"Включить диктовку" action:@selector(toggle:) keyEquivalent:@""];
  toggleItem.target = self;
  [menu addItem:[NSMenuItem separatorItem]];
  [menu addItemWithTitle:@"Начать: «Енот текст»" action:nil keyEquivalent:@""];
  [menu addItemWithTitle:@"Остановить: «Енот стоп»" action:nil keyEquivalent:@""];
  detailItem = [menu addItemWithTitle:@"Текст появляется после паузы" action:nil keyEquivalent:@""];
  menu.autoenablesItems = NO;
  for (NSMenuItem *entry in menu.itemArray) entry.enabled = NO;
  item.menu = menu;
  [self updateActive:NO ready:NO problem:@"" heard:@""];
 }
 return self;
}
- (void)toggle:(id)sender { toggleDictation(handle); }
- (void)updateActive:(BOOL)active ready:(BOOL)ready problem:(NSString *)problem heard:(NSString *)heard {
 BOOL failed = problem.length > 0;
 NSString *label = failed ? @"Ошибка" : (!ready ? @"Загрузка…" : (active ? @"ВКЛ" : @"ожидание"));
 item.button.title = [@" Енот: " stringByAppendingString:label];
 item.button.toolTip = failed ? problem : (active ? @"Диктовка включена" : @"Ожидание команды «Енот текст»");
 item.button.image = [NSImage imageWithSystemSymbolName:(failed ? @"exclamationmark.triangle" : (active ? @"mic.fill" : @"mic")) accessibilityDescription:label];
 stateItem.title = failed ? @"Ошибка голосового ввода" : (!ready ? @"Загрузка GigaAM…" : (active ? @"Диктовка включена" : @"Ожидание команды"));
 toggleItem.title = active ? @"Остановить диктовку" : @"Включить диктовку";
 toggleItem.enabled = ready;
 detailItem.title = failed ? problem : (heard.length ? [@"Услышано: " stringByAppendingString:heard] : @"Текст появляется после паузы");
}
- (void)dealloc {
 [[NSStatusBar systemStatusBar] removeStatusItem:item];
 [item release];
 [super dealloc];
}
@end

void *voice_status_create(uintptr_t handle) {
 @autoreleasepool {
  [NSApplication sharedApplication];
  [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
  return [[VoiceStatus alloc] initWithHandle:handle];
 }
}
void voice_status_update(void *status, int active, int ready, const char *problem, const char *heard) {
 @autoreleasepool {
  VoiceStatus *view = (VoiceStatus *)status;
  NSString *message = [NSString stringWithUTF8String:problem];
  NSString *recognition = [NSString stringWithUTF8String:heard];
  dispatch_async(dispatch_get_main_queue(), ^{ [view updateActive:active ready:ready problem:message heard:recognition]; });
 }
}
void voice_status_run(void *status) {
 @autoreleasepool { [NSApp run]; [(VoiceStatus *)status release]; }
}
void voice_status_stop(void) {
 dispatch_async(dispatch_get_main_queue(), ^{
  [NSApp stop:nil];
  [NSApp postEvent:[NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSZeroPoint modifierFlags:0 timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0] atStart:NO];
 });
}
