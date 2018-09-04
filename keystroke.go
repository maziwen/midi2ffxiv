// +build windows

/*
   MIDI2FFXIV
   Copyright (C) 2017-2018 Star Brilliant <m13253@hotmail.com>

   Permission is hereby granted, free of charge, to any person obtaining a
   copy of this software and associated documentation files (the "Software"),
   to deal in the Software without restriction, including without limitation
   the rights to use, copy, modify, merge, publish, distribute, sublicense,
   and/or sell copies of the Software, and to permit persons to whom the
   Software is furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in
   all copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
   FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
   DEALINGS IN THE SOFTWARE.
*/

package main

import (
	"fmt"
	"log"
	"time"

	"./user32"
	cgc "github.com/m13253/cgc-go"
)

type keystrokeStatus struct {
	pressedKeys         [256]bool
	pressedKeysCount    int
	isCtrlDown          bool
	isAltDown           bool
	isShiftDown         bool
	clearModifiersTimer *time.Timer
}

func (app *application) processKeystrokes() {
	app.keyStatus = &keystrokeStatus{
		clearModifiersTimer: time.NewTimer(app.IdleDuration),
	}
	for {
		select {
		case r, ok := <-app.KeystrokeGoro:
			if !ok {
				return
			}
			_ = cgc.RunOneRequest(app.ctx, r)
		case <-app.keyStatus.clearModifiersTimer.C:
			app.clearModifiers()
		case <-app.ctx.Done():
			return
		}
	}
}

func (app *application) produceKeystroke(note *midiMessage) {
	pInputs := []user32.INPUT_KEYBDINPUT{}
	if note.Msg[0] == 0x90 {
		keybind := app.Keybinding[int(note.Msg[1])]
		if app.keyStatus.pressedKeys[keybind.VirtualKeyCode] {
			pInputs = append(pInputs, user32.INPUT_KEYBDINPUT{
				Type: user32.INPUT_KEYBOARD,
				Ki: user32.KEYBDINPUT{
					WVk:         0,
					WScan:       uint16(user32.MapVirtualKey(uint32(keybind.VirtualKeyCode), user32.MAPVK_VK_TO_VSC)),
					DwFlags:     user32.KEYEVENTF_SCANCODE | user32.KEYEVENTF_KEYUP,
					Time:        0,
					DwExtraInfo: 0,
				},
			})
			app.keyStatus.pressedKeysCount--
		}
		if app.keyStatus.isCtrlDown != keybind.Ctrl {
			dwFlags := user32.KEYEVENTF_SCANCODE | user32.KEYEVENTF_KEYUP
			if keybind.Ctrl {
				dwFlags = user32.KEYEVENTF_SCANCODE
			}
			pInputs = append(pInputs, user32.INPUT_KEYBDINPUT{
				Type: user32.INPUT_KEYBOARD,
				Ki: user32.KEYBDINPUT{
					WVk:         0,
					WScan:       uint16(user32.MapVirtualKey(uint32(user32.VK_CONTROL), user32.MAPVK_VK_TO_VSC)),
					DwFlags:     dwFlags,
					Time:        0,
					DwExtraInfo: 0,
				},
			})
			app.keyStatus.isCtrlDown = keybind.Ctrl
		}
		if app.keyStatus.isAltDown != keybind.Alt {
			dwFlags := user32.KEYEVENTF_SCANCODE | user32.KEYEVENTF_KEYUP
			if keybind.Alt {
				dwFlags = user32.KEYEVENTF_SCANCODE
			}
			pInputs = append(pInputs, user32.INPUT_KEYBDINPUT{
				Type: user32.INPUT_KEYBOARD,
				Ki: user32.KEYBDINPUT{
					WVk:         0,
					WScan:       uint16(user32.MapVirtualKey(uint32(user32.VK_MENU), user32.MAPVK_VK_TO_VSC)),
					DwFlags:     dwFlags,
					Time:        0,
					DwExtraInfo: 0,
				},
			})
			app.keyStatus.isAltDown = keybind.Alt
		}
		if app.keyStatus.isShiftDown != keybind.Shift {
			dwFlags := user32.KEYEVENTF_SCANCODE | user32.KEYEVENTF_KEYUP
			if keybind.Shift {
				dwFlags = user32.KEYEVENTF_SCANCODE
			}
			pInputs = append(pInputs, user32.INPUT_KEYBDINPUT{
				Type: user32.INPUT_KEYBOARD,
				Ki: user32.KEYBDINPUT{
					WVk:         0,
					WScan:       uint16(user32.MapVirtualKey(uint32(user32.VK_SHIFT), user32.MAPVK_VK_TO_VSC)),
					DwFlags:     dwFlags,
					Time:        0,
					DwExtraInfo: 0,
				},
			})
			app.keyStatus.isShiftDown = keybind.Shift
		}
		pInputs = append(pInputs, user32.INPUT_KEYBDINPUT{
			Type: user32.INPUT_KEYBOARD,
			Ki: user32.KEYBDINPUT{
				WVk:         0,
				WScan:       uint16(user32.MapVirtualKey(uint32(keybind.VirtualKeyCode), user32.MAPVK_VK_TO_VSC)),
				DwFlags:     user32.KEYEVENTF_SCANCODE,
				Time:        0,
				DwExtraInfo: 0,
			},
		})
		app.keyStatus.pressedKeys[keybind.VirtualKeyCode] = true
		app.keyStatus.pressedKeysCount++
		app.keyStatus.clearModifiersTimer.Stop()
	} else if note.Msg[0] == 0x80 {
		keybind := app.Keybinding[int(note.Msg[1])]
		if app.keyStatus.pressedKeys[keybind.VirtualKeyCode] {
			pInputs = append(pInputs, user32.INPUT_KEYBDINPUT{
				Type: user32.INPUT_KEYBOARD,
				Ki: user32.KEYBDINPUT{
					WVk:         0,
					WScan:       uint16(user32.MapVirtualKey(uint32(keybind.VirtualKeyCode), user32.MAPVK_VK_TO_VSC)),
					DwFlags:     user32.KEYEVENTF_SCANCODE | user32.KEYEVENTF_KEYUP,
					Time:        0,
					DwExtraInfo: 0,
				},
			})
			app.keyStatus.pressedKeys[keybind.VirtualKeyCode] = false
			app.keyStatus.pressedKeysCount--
		}
		if app.keyStatus.pressedKeysCount == 0 {
			app.keyStatus.clearModifiersTimer.Reset(app.IdleDuration)
		}
	}
	if len(pInputs) != 0 {
		for i := range pInputs {
			_, err := user32.SendInput(pInputs[i : i+1])
			if err != nil {
				fmt.Println("Error: ", err)
			}
		}
		app.printPressedKeys()
	}
}

func (app *application) clearModifiers() {
	pInputs := []user32.INPUT_KEYBDINPUT{}
	if app.keyStatus.isCtrlDown {
		pInputs = append(pInputs, user32.INPUT_KEYBDINPUT{
			Type: user32.INPUT_KEYBOARD,
			Ki: user32.KEYBDINPUT{
				WVk:         0,
				WScan:       uint16(user32.MapVirtualKey(uint32(user32.VK_CONTROL), user32.MAPVK_VK_TO_VSC)),
				DwFlags:     user32.KEYEVENTF_SCANCODE | user32.KEYEVENTF_KEYUP,
				Time:        0,
				DwExtraInfo: 0,
			},
		})
		app.keyStatus.isCtrlDown = false
	}
	if app.keyStatus.isAltDown {
		pInputs = append(pInputs, user32.INPUT_KEYBDINPUT{
			Type: user32.INPUT_KEYBOARD,
			Ki: user32.KEYBDINPUT{
				WVk:         0,
				WScan:       uint16(user32.MapVirtualKey(uint32(user32.VK_MENU), user32.MAPVK_VK_TO_VSC)),
				DwFlags:     user32.KEYEVENTF_SCANCODE | user32.KEYEVENTF_KEYUP,
				Time:        0,
				DwExtraInfo: 0,
			},
		})
		app.keyStatus.isAltDown = false
	}
	if app.keyStatus.isShiftDown {
		pInputs = append(pInputs, user32.INPUT_KEYBDINPUT{
			Type: user32.INPUT_KEYBOARD,
			Ki: user32.KEYBDINPUT{
				WVk:         0,
				WScan:       uint16(user32.MapVirtualKey(uint32(user32.VK_SHIFT), user32.MAPVK_VK_TO_VSC)),
				DwFlags:     user32.KEYEVENTF_SCANCODE | user32.KEYEVENTF_KEYUP,
				Time:        0,
				DwExtraInfo: 0,
			},
		})
		app.keyStatus.isShiftDown = false
	}
	if len(pInputs) != 0 {
		for i := range pInputs {
			_, err := user32.SendInput(pInputs[i : i+1])
			if err != nil {
				fmt.Println("Error: ", err)
			}
		}
		app.printPressedKeys()
	}
}

func (app *application) printPressedKeys() {
	pressedKeysCount := 0
	line := "["
	if app.keyStatus.isCtrlDown {
		line += " Ctrl"
	}
	if app.keyStatus.isAltDown {
		line += " Alt"
	}
	if app.keyStatus.isShiftDown {
		line += " Shift"
	}
	for i, v := range app.keyStatus.pressedKeys {
		if v {
			line += fmt.Sprintf(" %q", rune(i))
			pressedKeysCount++
		}
	}
	line += " ]"
	log.Println(line)
	if pressedKeysCount != app.keyStatus.pressedKeysCount {
		panic(fmt.Sprintf("pressedKeysCount (%d) != app.pressedKeysCount (%d)", pressedKeysCount, app.keyStatus.pressedKeysCount))
	}
}
