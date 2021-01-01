// Erase the space before "go" to enable generating the version info from the version info file when it's in the root directory
// go:generate goversioninfo -64=true
package main

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	//"strings"
	"time"
	"encoding/json"

	//"github.com/adrg/xdg"
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/tanema/gween"
  "github.com/inkyblackness/imgui-go/v3"

	"io/ioutil"
	"os"

	"github.com/adrg/xdg"
	"github.com/tidwall/gjson"
)

// Build-time variable
var releaseMode = "false"

var camera = rl.NewCamera2D(rl.Vector2{480, 270}, rl.Vector2{}, 0, 1)
var currentProject *Project
var drawFPS = false
var spacing = float32(1)
var lineSpacing = float32(1) // This is assuming font size is the height, which it is for my font
var font rl.Font
var not_shit_font rl.Font
var windowTitle = "notMasterPlan"
var softwareVersion = 1
var deltaTime = float32(0)
var quit = false

const SETTINGS_PATH = "MasterPlan/settings.json"

type ProgramSettings struct {
	RecentPlanList            []string
	AutoloadLastPlan          bool
	WindowPosition            rl.Rectangle
	SaveWindowPosition        bool
	Keybindings               *Keybindings
}

var programSettings = ProgramSettings{
  RecentPlanList:         []string{},
  WindowPosition:         rl.NewRectangle(-1, -1, 0, 0),
  SaveWindowPosition:     true,
  Keybindings:            NewKeybindings(),
}

func (ps *ProgramSettings) CleanUpRecentPlanList() {

	newList := []string{}
	for i, s := range ps.RecentPlanList {
		_, err := os.Stat(s)
		if err == nil {
			newList = append(newList, ps.RecentPlanList[i]) // We could alter the slice to cut out the strings that are invalid, but this is visually cleaner and easier to understand
		}
	}
	ps.RecentPlanList = newList
}

func (ps *ProgramSettings) Save() {

	path, _ := xdg.ConfigFile(SETTINGS_PATH)
	f, err := os.Create(path)
	if err == nil {
		defer f.Close()
		bytes, _ := json.Marshal(ps)
		f.Write([]byte(gjson.Parse(string(bytes)).Get("@pretty").String()))
		f.Sync()
	}
}

type EventLog struct {
	Time  time.Time
	Text  string
	Tween *gween.Tween
}

var eventLogBuffer = []EventLog{}

func main() {

	// We want to defer a function to recover out of a crash if in release mode.
	// We do this because by default, Go's stderr points directly to the OS's syserr buffer.
	// By deferring this function and recovering out of the crash, we can grab the crashlog by
	// using runtime.Caller().

	defer func() {
		if releaseMode == "true" {
			panicOut := recover()
			if panicOut != nil {

				log.Print(
					"# ERROR START #\n",
				)

				stackContinue := true
				i := 0 // We can skip the first few crash lines, as they reach up through the main
				// function call and into this defer() call.
				for stackContinue {
					// Recover the lines of the crash log and log it out.
					_, fn, line, ok := runtime.Caller(i)
					stackContinue = ok
					if ok {
						fmt.Print("\n", fn, ":", line)
						if i == 0 {
							fmt.Print(" | ", "Error: ", panicOut)
						}
						i++
					}
				}

				fmt.Print(
					"\n\n# ERROR END #\n",
				)
			}
		}
	}()

	rl.SetTraceLog(rl.LogError)

  {
    path, _ := xdg.ConfigFile(SETTINGS_PATH)
    settingsJSON, err := ioutil.ReadFile(path)

    if err == nil {
      json.Unmarshal(settingsJSON, programSettings)
    }
  }

	windowFlags := byte(rl.FlagWindowResizable)

	rl.SetConfigFlags(windowFlags)

	// We initialize the window using just "MasterPlan" as the title because WM_CLASS is set from this on Linux
	rl.InitWindow(960, 540, "MasterPlan")

	rl.SetWindowIcon(*rl.LoadImage(GetPath("assets", "window_icon.png")))

	if programSettings.SaveWindowPosition && programSettings.WindowPosition.Width > 0 && programSettings.WindowPosition.Height > 0 {
		rl.SetWindowPosition(int(programSettings.WindowPosition.X), int(programSettings.WindowPosition.Y))
		rl.SetWindowSize(int(programSettings.WindowPosition.Width), int(programSettings.WindowPosition.Height))
	}

	ReloadFonts()

	currentProject = NewProject()

	rl.SetExitKey(0) /// We don't want Escape to close the program.

	fpsDisplayValue := float32(0)
	fpsDisplayAccumulator := float32(0)
	fpsDisplayTimer := time.Now()

	elapsed := time.Duration(0)

	log.Println("MasterPlan initialized successfully.")

  imgui.CreateContext(nil)
  ImGui_ImplRaylib_Init()
  imrend, _ := NewOpenGL3(imgui.CurrentIO())

	for !rl.WindowShouldClose() && !quit {

		currentTime := time.Now()

		handleMouseInputs()

		if rl.IsKeyPressed(rl.KeyF1) {
			drawFPS = !drawFPS
		}

    ImGui_ImplRaylib_ProcessEvent()
    ImGui_ImplRaylib_NewFrame()
    imgui.NewFrame()

		clearColor := getThemeColor(GUI_INSIDE_DISABLED)

		if windowFlags&byte(rl.FlagWindowTransparent) > 0 {
			clearColor = rl.Color{}
		}

		rl.ClearBackground(clearColor)

		rl.BeginDrawing()

    rl.BeginMode2D(camera)

    currentProject.Update()

    rl.EndMode2D()

    color := getThemeColor(GUI_FONT_COLOR)
    color.A = 128

    //x := float32(rl.GetScreenWidth() - 8)
    v := ""

    if currentProject.Modified {
      v += "Modified"
    }

    if len(v) > 0 {
      // TODO(justasd): :Text
      //x -= GUITextWidth(v)
      //DrawGUITextColored(rl.Vector2{x, 8}, color, v)
    }

    color = rl.White
    //bgColor := rl.Black

    //y := float32(24)

    // TODO(justasd): :Text
    /*
    {
      for i := 0; i < len(eventLogBuffer); i++ {

        msg := eventLogBuffer[i]

        text := "- " + msg.Time.Format("15:04:05") + " : " + msg.Text
        text = strings.ReplaceAll(text, "\n", "\n                    ")

        alpha, done := msg.Tween.Update(rl.GetFrameTime())
        color.A = uint8(alpha)
        bgColor.A = color.A

        textSize := rl.MeasureTextEx(font, text, float32(GUIFontSize()), 1)
        lineHeight, _ := TextHeight(text, true)
        textPos := rl.Vector2{8, y}
        rectPos := textPos

        rectPos.X--
        rectPos.Y--
        textSize.X += 2
        textSize.Y = lineHeight

        rl.DrawRectangleV(textPos, textSize, bgColor)
        DrawGUITextColored(textPos, color, text)

        if done {
          eventLogBuffer = append(eventLogBuffer[:i], eventLogBuffer[i+1:]...)
          i--
        }

        y += lineHeight

      }

    }
    */

    if drawFPS {
      rl.DrawTextEx(font, fmt.Sprintf("%.2f", fpsDisplayValue), rl.Vector2{0, 0}, 60, spacing, rl.Red)
    }

    {
      imgui.ShowDemoWindow(nil)

      imgui.Render()

      wnd_size_arr := [2]float32{float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight())}
      imrend.Render(wnd_size_arr, wnd_size_arr, imgui.RenderedDrawData())
    }

		rl.EndDrawing()

		title := windowTitle

		if currentProject.FilePath != "" {
			_, fileName := filepath.Split(currentProject.FilePath)
			title += fmt.Sprintf(" - %s", fileName)
		}

		if currentProject.Modified {
			title += " *"
		}

		if windowTitle != title {
			rl.SetWindowTitle(title)
			windowTitle = title
		}

		targetFPS := 144

		if !rl.IsWindowFocused() || rl.IsWindowHidden() || rl.IsWindowMinimized() {
			targetFPS = 10
		}

		elapsed += time.Since(currentTime)
		attemptedSleep := (time.Second / time.Duration(targetFPS)) - elapsed

		beforeSleep := time.Now()
		time.Sleep(attemptedSleep)
		sleepDifference := time.Since(beforeSleep) - attemptedSleep

		if attemptedSleep > 0 {
			deltaTime = float32((attemptedSleep + elapsed).Seconds())
		} else {
			sleepDifference = 0
			deltaTime = float32(elapsed.Seconds())
		}

		if time.Since(fpsDisplayTimer).Seconds() >= 1 {
			fpsDisplayTimer = time.Now()
			fpsDisplayValue = fpsDisplayAccumulator * float32(targetFPS)
			fpsDisplayAccumulator = 0
		}
		fpsDisplayAccumulator += 1.0 / float32(targetFPS)

		elapsed = sleepDifference // Sleeping doesn't sleep for exact amounts; carry this into next frame for sleep attempt
	}

	if programSettings.SaveWindowPosition {
		// This is outside the main loop because we can save the window properties just before quitting
		wp := rl.GetWindowPosition()
		wr := rl.Rectangle{wp.X, wp.Y, float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight())}
		programSettings.WindowPosition = wr
		programSettings.Save()
	}

	log.Println("MasterPlan exited successfully.")

	currentProject.Destroy()
}
