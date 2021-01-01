package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	GUI_OUTLINE             = "GUI_OUTLINE"
	GUI_OUTLINE_HIGHLIGHTED = "GUI_OUTLINE_HIGHLIGHTED"
	GUI_OUTLINE_DISABLED    = "GUI_OUTLINE_DISABLED"
	GUI_INSIDE              = "GUI_INSIDE"
	GUI_INSIDE_HIGHLIGHTED  = "GUI_INSIDE_HIGHLIGHTED"
	GUI_INSIDE_DISABLED     = "GUI_INSIDE_DISABLED"
	GUI_FONT_COLOR          = "GUI_FONT_COLOR"
	GUI_NOTE_COLOR          = "GUI_NOTE_COLOR"
	GUI_SHADOW_COLOR        = "GUI_SHADOW_COLOR"
)

var currentTheme = "Sunlight" // Default theme for new projects and new sessions is the Sunlight theme

var guiColors map[string]map[string]rl.Color

var worldGUI = false // Controls whether to use world coordinates for input and rendering

func getThemeColor(colorConstant string) rl.Color {
	return guiColors[currentTheme][colorConstant]
}

func loadThemes() {

	newGUIColors := map[string]map[string]rl.Color{}

	filepath.Walk(GetPath("assets", "themes"), func(fp string, info os.FileInfo, err error) error {

		if !info.IsDir() {

			themeFile, err := os.Open(fp)

			if err == nil {

				defer themeFile.Close()

				_, themeName := filepath.Split(fp)
				themeName = strings.Split(themeName, ".json")[0]

				// themeData := []byte{}
				themeData := ""
				var jsonData map[string][]uint8

				scanner := bufio.NewScanner(themeFile)
				for scanner.Scan() {
					// themeData = append(themeData, scanner.Bytes()...)
					themeData += scanner.Text()
				}
				json.Unmarshal([]byte(themeData), &jsonData)

				// A length of 0 means JSON couldn't properly unmarshal the data, so it was mangled somehow.
				if len(jsonData) > 0 {

					newGUIColors[themeName] = map[string]rl.Color{}

					for key, value := range jsonData {
						if !strings.Contains(key, "//") { // Strings that begin with "//" are ignored
							newGUIColors[themeName][key] = rl.Color{value[0], value[1], value[2], value[3]}
						}
					}

				} else {
					newGUIColors[themeName] = guiColors[themeName]
				}

			}
		}
		if err != nil {
			return err
		}
		return nil
	})

	guiColors = newGUIColors

}

/*
// TextHeight returns the height of the text, as well as how many lines are in the provided text.
func TextHeight(text string, usingGuiFont bool) (float32, int) {
	nCount := strings.Count(text, "\n") + 1
	totalHeight := float32(0)
	if usingGuiFont {
		totalHeight = float32(nCount) * lineSpacing * GUIFontSize()
	} else {
		totalHeight = float32(nCount) * lineSpacing * float32(programSettings.FontSize)
	}
	return totalHeight, nCount

}


func GUITextWidth(text string) float32 {
	w := float32(0)
	for _, c := range text {
		w += rl.MeasureTextEx(font, string(c), GUIFontSize(), spacing).X + spacing
	}
	return w
}

func TextSize(text string, guiText bool) (rl.Vector2, int) {

	nCount := strings.Count(text, "\n") + 1

	fs := float32(programSettings.FontSize)

	if guiText {
		fs = GUIFontSize()
	}

	size := rl.MeasureTextEx(font, text, fs, spacing)

	if guiText {
		size.Y = float32(nCount) * lineSpacing * GUIFontSize()
	} else {
		size.Y = float32(nCount) * lineSpacing * float32(programSettings.FontSize)
	}

	return size, nCount

}

func DrawTextColored(pos rl.Vector2, fontColor rl.Color, text string, guiMode bool, variables ...interface{}) {

	if len(variables) > 0 {
		text = fmt.Sprintf(text, variables...)
	}
	pos.Y -= 2 // Text is a bit low

	size := float32(programSettings.FontSize)
	f := font

	if guiMode {
		size = float32(GUIFontSize())
		f = font
	}

	height, lineCount := TextHeight(text, guiMode)

	pos.X = float32(int32(pos.X))
	pos.Y = float32(int32(pos.Y))

	for _, line := range strings.Split(text, "\n") {
		rl.DrawTextEx(f, line, pos, size, spacing, fontColor)
		pos.Y += float32(int32(height / float32(lineCount)))
	}

}

func DrawText(pos rl.Vector2, text string, values ...interface{}) {
	DrawTextColored(pos, getThemeColor(GUI_FONT_COLOR), text, false, values...)
}

func DrawGUIText(pos rl.Vector2, text string, values ...interface{}) {
	DrawTextColored(pos, getThemeColor(GUI_FONT_COLOR), text, true, values...)
}

func DrawGUITextColored(pos rl.Vector2, fontColor rl.Color, text string, values ...interface{}) {
	DrawTextColored(pos, fontColor, text, true, values...)
}
*/
