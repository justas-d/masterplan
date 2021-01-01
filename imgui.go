package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
  "github.com/inkyblackness/imgui-go/v3"
)

type imclipboard struct {
}

func (cb *imclipboard) Text() (string, error) {
  return rl.GetClipboardText(), nil
}

func (cb *imclipboard) SetText(value string) {
  rl.SetClipboardText(value);
}

func ImGui_ImplRaylib_Init() {
  io := imgui.CurrentIO()

  io.KeyMap(imgui.KeyTab, rl.KeyTab)
  io.KeyMap(imgui.KeyLeftArrow, rl.KeyLeft)
  io.KeyMap(imgui.KeyRightArrow, rl.KeyRight)
  io.KeyMap(imgui.KeyUpArrow, rl.KeyUp)
  io.KeyMap(imgui.KeyDownArrow, rl.KeyDown)
  io.KeyMap(imgui.KeyPageUp, rl.KeyPageUp)
  io.KeyMap(imgui.KeyPageDown, rl.KeyPageDown)
  io.KeyMap(imgui.KeyHome, rl.KeyHome)
  io.KeyMap(imgui.KeyEnd, rl.KeyEnd)
  io.KeyMap(imgui.KeyInsert, rl.KeyInsert)
  io.KeyMap(imgui.KeyDelete, rl.KeyDelete)
  io.KeyMap(imgui.KeyBackspace, rl.KeyBackspace)
  io.KeyMap(imgui.KeySpace, rl.KeySpace)
  io.KeyMap(imgui.KeyEnter, rl.KeyEnter)
  io.KeyMap(imgui.KeyEscape, rl.KeyEscape)
  io.KeyMap(imgui.KeyKeyPadEnter, rl.KeyKpEnter)
  io.KeyMap(imgui.KeyA, rl.KeyA)
  io.KeyMap(imgui.KeyC, rl.KeyC)
  io.KeyMap(imgui.KeyV, rl.KeyV)
  io.KeyMap(imgui.KeyX, rl.KeyX)
  io.KeyMap(imgui.KeyY, rl.KeyY)
  io.KeyMap(imgui.KeyZ, rl.KeyZ);

  clip := imclipboard{}
  io.SetClipboard(&clip)
}

var gtime float32

func ImGui_ImplRaylib_NewFrame() {
  io := imgui.CurrentIO()

  io.SetDisplaySize(imgui.Vec2{ float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight()) })

  current_time := rl.GetTime()

  set_time := float32(0.0)
  if gtime > 0.0 {
    set_time = current_time - gtime
  } else {
    set_time = 1.0/60.0
  }

  io.SetDeltaTime(set_time)
  gtime = current_time

  toint:= func(v bool) int {
    if v {
      return 1
    }
    return 0
  }

  io.KeyCtrl(toint(rl.IsKeyDown(rl.KeyLeftControl)), toint(rl.IsKeyDown(rl.KeyRightControl)))
  io.KeyShift(toint(rl.IsKeyDown(rl.KeyLeftShift)), toint(rl.IsKeyDown(rl.KeyRightShift)))
  io.KeyAlt(toint(rl.IsKeyDown(rl.KeyLeftAlt)), toint(rl.IsKeyDown(rl.KeyRightAlt)))
  io.KeySuper(toint(rl.IsKeyDown(rl.KeyLeftSuper)), toint(rl.IsKeyDown(rl.KeyRightSuper)))

  io.SetMouseButtonDown(0, rl.IsMouseButtonDown(rl.MouseLeftButton))
  io.SetMouseButtonDown(1, rl.IsMouseButtonDown(rl.MouseRightButton))
  io.SetMouseButtonDown(2, rl.IsMouseButtonDown(rl.MouseMiddleButton))

  if !rl.IsWindowMinimized() {
    io.SetMousePosition(imgui.Vec2{float32(rl.GetMouseX()), float32(rl.GetMouseY())})
  }


  if rl.GetMouseWheelMove() > 0 {
    io.AddMouseWheelDelta(0.0, 1.0)
  }  else if rl.GetMouseWheelMove() < 0 {
    io.AddMouseWheelDelta(0.0, -1.0)
  }
}

func ImGui_ImplRaylib_ProcessEvent() {
  keys := []int {
    rl.KeyApostrophe,
    rl.KeyComma,
    rl.KeyMinus,
    rl.KeyPeriod,
    rl.KeySlash,
    rl.KeyZero,
    rl.KeyOne,
    rl.KeyTwo,
    rl.KeyThree,
    rl.KeyFour,
    rl.KeyFive,
    rl.KeySix,
    rl.KeySeven,
    rl.KeyEight,
    rl.KeyNine,
    rl.KeySemicolon,
    rl.KeyEqual,
    rl.KeyA,
    rl.KeyB,
    rl.KeyC,
    rl.KeyD,
    rl.KeyE,
    rl.KeyF,
    rl.KeyG,
    rl.KeyH,
    rl.KeyI,
    rl.KeyJ,
    rl.KeyK,
    rl.KeyL,
    rl.KeyM,
    rl.KeyN,
    rl.KeyO,
    rl.KeyP,
    rl.KeyQ,
    rl.KeyR,
    rl.KeyS,
    rl.KeyT,
    rl.KeyU,
    rl.KeyV,
    rl.KeyW,
    rl.KeyX,
    rl.KeyY,
    rl.KeyZ,
    rl.KeySpace,
    rl.KeyEscape,
    rl.KeyEnter,
    rl.KeyTab,
    rl.KeyBackspace,
    rl.KeyInsert,
    rl.KeyDelete,
    rl.KeyRight,
    rl.KeyLeft,
    rl.KeyDown,
    rl.KeyUp,
    rl.KeyPageUp,
    rl.KeyPageDown,
    rl.KeyHome,
    rl.KeyEnd,
    rl.KeyCapsLock,
    rl.KeyScrollLock,
    rl.KeyNumLock,
    rl.KeyPrintScreen,
    rl.KeyPause,
    rl.KeyF1,
    rl.KeyF2,
    rl.KeyF3,
    rl.KeyF4,
    rl.KeyF5,
    rl.KeyF6,
    rl.KeyF7,
    rl.KeyF8,
    rl.KeyF9,
    rl.KeyF10,
    rl.KeyF11,
    rl.KeyF12,
    rl.KeyLeftShift,
    rl.KeyLeftControl,
    rl.KeyLeftAlt,
    rl.KeyLeftSuper,
    rl.KeyRightShift,
    rl.KeyRightControl,
    rl.KeyRightAlt,
    rl.KeyRightSuper,
    rl.KeyKbMenu,
    rl.KeyLeftBracket,
    rl.KeyBackSlash,
    rl.KeyRightBracket,
    rl.KeyGrave,
    rl.KeyKp0,
    rl.KeyKp1,
    rl.KeyKp2,
    rl.KeyKp3,
    rl.KeyKp4,
    rl.KeyKp5,
    rl.KeyKp6,
    rl.KeyKp7,
    rl.KeyKp8,
    rl.KeyKp9,
    rl.KeyKpDecimal,
    rl.KeyKpDivide,
    rl.KeyKpMultiply,
    rl.KeyKpSubtract,
    rl.KeyKpAdd,
    rl.KeyKpEnter,
    rl.KeyKpEqual,
  }

  io := imgui.CurrentIO()

  for _, key := range keys {
    if rl.IsKeyDown(int32(key)) {
      io.KeyPress(key)
    }
    if rl.IsKeyUp(int32(key)) {
      io.KeyRelease(key)
    }
  }

  io.AddInputCharacters(string(rl.GetKeyPressed()));
}
