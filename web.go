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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

type webHandlers struct {
	app *application

	server   *http.Server
	serveMux *http.ServeMux
}

type basicAuthHandler struct {
	Handler  http.Handler
	Username string
	Password string
}

func (app *application) startWebServer() error {
	h := &webHandlers{
		app:      app,
		server:   new(http.Server),
		serveMux: http.NewServeMux(),
	}
	h.server.Handler = &basicAuthHandler{
		Handler:  h.serveMux,
		Username: h.app.WebUsername,
		Password: h.app.WebPassword,
	}
	h.serveMux.Handle("/", http.FileServer(http.Dir("web")))
	h.serveMux.HandleFunc("/version", h.version)
	h.serveMux.HandleFunc("/midi-input-device", h.midiInputDevice)
	h.serveMux.HandleFunc("/midi-output-device", h.midiOutputDevice)
	h.serveMux.HandleFunc("/midi-output-bank", h.midiOutputBank)
	h.serveMux.HandleFunc("/midi-output-patch", h.midiOutputPatch)
	h.serveMux.HandleFunc("/midi-output-transpose", h.midiOutputTranspose)
	h.serveMux.HandleFunc("/current-time", h.currentTime)
	h.serveMux.HandleFunc("/ntp-sync-server", h.ntpSyncServer)
	h.serveMux.HandleFunc("/midi-playback-file", h.midiPlaybackFile)
	h.serveMux.HandleFunc("/midi-playback-track", h.midiPlaybackTrack)
	h.serveMux.HandleFunc("/midi-playback-offset", h.midiPlaybackOffset)
	h.serveMux.HandleFunc("/scheduler", h.scheduler)

	originalAddr, err := net.ResolveTCPAddr("tcp", app.WebListenAddr)
	availableAddr := new(net.TCPAddr)
	*availableAddr = *originalAddr
	if err != nil {
		return err
	}

	var l net.Listener
	for availableAddr.Port = originalAddr.Port; availableAddr.Port < 65535 && availableAddr.Port-originalAddr.Port < 10; availableAddr.Port++ {
		l, err = net.ListenTCP("tcp", availableAddr)
		if err != nil {
			if isErrorAddressAlreadyInUse(err) {
				continue
			} else {
				return err
			}
		}
		break
	}

	h.server.Addr = availableAddr.String()
	if len(availableAddr.IP) == 0 || availableAddr.IP.IsUnspecified() {
		fmt.Printf("\nPlease open the control panel at http://localhost:%d\n\n", availableAddr.Port)
	} else {
		fmt.Printf("\nPlease open the control panel at http://%s\n\n", h.server.Addr)
	}
	if h.app.WebUsername != "" || h.app.WebPassword != "" {
		fmt.Printf("Username: %s\nPassword: %s\n\n", h.app.WebUsername, h.app.WebPassword)
	}

	go h.waitForQuit()
	go func() {
		h.server.Serve(l)
		l.Close()
	}()

	return nil
}

func isErrorAddressAlreadyInUse(err error) bool {
	errOpError, ok := err.(*net.OpError)
	if !ok {
		return false
	}
	errSyscallError, ok := errOpError.Err.(*os.SyscallError)
	if !ok {
		return false
	}
	errErrno, ok := errSyscallError.Err.(syscall.Errno)
	if !ok {
		return false
	}
	if errErrno == syscall.EADDRINUSE {
		return true
	}
	const WSAEADDRINUSE = 10048
	if runtime.GOOS == "windows" && errErrno == WSAEADDRINUSE {
		return true
	}
	return false
}

func (h *webHandlers) waitForQuit() {
	<-h.app.ctx.Done()
	h.server.Close()
}

func (h *webHandlers) version(w http.ResponseWriter, r *http.Request) {
	var result struct {
		VersionInfo string `json:"version_info"`
	}
	result.VersionInfo = versionInfo
	writeJSON(w, result)
}

func (h *webHandlers) midiInputDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method == "PUT" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 500)
			return
		}
		value, err := strconv.ParseInt(string(body), 0, 32)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 400)
			return
		}
		_, err = h.app.MidiRealtimeGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
			return nil, h.app.openMidiInDevice(int(value))
		})
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 503)
			return
		}
	}

	var result struct {
		Devices  []string `json:"devices"`
		Selected int      `json:"selected"`
	}
	h.app.MidiRealtimeGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
		result.Devices = h.app.listMidiInDevices()
		result.Selected = h.app.MidiInDevice
		return nil, nil
	})
	writeJSON(w, result)
}

func (h *webHandlers) midiOutputDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method == "PUT" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 500)
			return
		}
		value, err := strconv.ParseInt(string(body), 0, 32)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 400)
			return
		}
		_, err = h.app.MidiRealtimeGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
			return nil, h.app.openMidiOutDevice(int(value))
		})
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 503)
			return
		}
	}

	var result struct {
		Devices  []string `json:"devices"`
		Selected int      `json:"selected"`
	}
	h.app.MidiRealtimeGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
		result.Devices = h.app.listMidiOutDevices()
		result.Selected = h.app.MidiOutDevice
		return nil, nil
	})
	writeJSON(w, result)
}

func (h *webHandlers) midiOutputBank(w http.ResponseWriter, r *http.Request) {
	if r.Method == "PUT" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 500)
			return
		}
		value, err := strconv.ParseUint(string(body), 0, 14)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 400)
			return
		}
		_, _ = h.app.MidiRealtimeGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
			h.app.setMidiOutBank(uint16(value))
			return nil, nil
		})
	}

	var result struct {
		Bank uint16 `json:"bank"`
	}
	h.app.MidiRealtimeGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
		result.Bank = h.app.MidiOutBank
		return nil, nil
	})
	writeJSON(w, result)
}

func (h *webHandlers) midiOutputPatch(w http.ResponseWriter, r *http.Request) {
	if r.Method == "PUT" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 500)
			return
		}
		value, err := strconv.ParseUint(string(body), 0, 7)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 400)
			return
		}
		_, _ = h.app.MidiRealtimeGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
			h.app.setMidiOutPatch(uint8(value))
			return nil, nil
		})
	}

	var result struct {
		Patch uint8 `json:"patch"`
	}
	h.app.MidiRealtimeGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
		result.Patch = h.app.MidiOutPatch
		return nil, nil
	})
	writeJSON(w, result)
}

func (h *webHandlers) midiOutputTranspose(w http.ResponseWriter, r *http.Request) {
	if r.Method == "PUT" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 500)
			return
		}
		value, err := strconv.ParseInt(string(body), 0, 8)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 400)
			return
		}
		_, err = h.app.MidiRealtimeGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
			h.app.setMidiOutTranspose(int(value))
			return nil, nil
		})
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 503)
			return
		}
	}

	var result struct {
		Transpose int `json:"transpose"`
	}
	h.app.MidiRealtimeGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
		result.Transpose = h.app.MidiOutTranspose
		return nil, nil
	})
	writeJSON(w, result)
}

func (h *webHandlers) currentTime(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	var result struct {
		Synced       bool    `json:"synced"`
		Time         float64 `json:"time"`
		MaxDeviation float64 `json:"max_deviation"`
	}
	synced, offset, maxDeviation := h.app.getNtpOffset()
	now = now.Add(offset).UTC()
	result.Synced = synced
	result.Time = float64(now.Unix()) + float64(now.Nanosecond())*1e-9
	result.MaxDeviation = float64(maxDeviation/time.Nanosecond) * 1e-9
	writeJSON(w, result)
}

func (h *webHandlers) ntpSyncServer(w http.ResponseWriter, r *http.Request) {
	if r.Method == "PUT" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 500)
			return
		}
		ntpServer := string(body)

		_, err = h.app.NtpGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
			return nil, h.app.syncTime(ntpServer)
		})
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 503)
			return
		}
		h.app.NtpSyncServer = ntpServer
	}

	var result struct {
		Server string `json:"server"`
	}
	result.Server = h.app.NtpSyncServer
	writeJSON(w, result)
}

func (h *webHandlers) midiPlaybackFile(w http.ResponseWriter, r *http.Request) {
	if r.Method == "PUT" {
		_, err := h.app.MidiPlaybackGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
			return nil, h.app.setMidiPlaybackFile(r.Body)
		})
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 503)
			return
		}

		writeJSON(w, struct{}{})
		return
	}

	http.Error(w, "Method Not Allowed", 405)
}

func (h *webHandlers) midiPlaybackTrack(w http.ResponseWriter, r *http.Request) {
	if r.Method == "PUT" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 500)
			return
		}
		value, err := strconv.ParseUint(string(body), 0, 16)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 400)
			return
		}
		_, err = h.app.MidiPlaybackGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
			h.app.setMidiPlaybackTrack(uint16(value))
			return nil, nil
		})
	}

	var result struct {
		Track uint16 `json:"track"`
	}
	h.app.MidiPlaybackGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
		result.Track = h.app.MidiPlaybackTrack
		return nil, nil
	})
	writeJSON(w, result)
}

func (h *webHandlers) midiPlaybackOffset(w http.ResponseWriter, r *http.Request) {
	if r.Method == "PUT" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 500)
			return
		}
		value, err := strconv.ParseFloat(string(body), 64)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 400)
			return
		}
		_, err = h.app.MidiPlaybackGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
			h.app.setMidiPlaybackOffset(time.Duration(value*1e9) * time.Nanosecond)
			return nil, nil
		})
	}

	var result struct {
		Offset float64 `json:"offset"`
	}
	h.app.MidiPlaybackGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
		result.Offset = float64(h.app.MidiPlaybackOffset/time.Nanosecond) * 1e-9
		return nil, nil
	})
	writeJSON(w, result)
}

func (h *webHandlers) scheduler(w http.ResponseWriter, r *http.Request) {
	var result struct {
		Enabled      bool     `json:"enabled"`
		StartTime    *float64 `json:"start_time"`
		LoopEnabled  bool     `json:"loop_enabled"`
		LoopInterval float64  `json:"loop_interval"`
	}

	if r.Method == "PUT" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 500)
			return
		}
		err = json.Unmarshal(body, &result)
		if err != nil {
			log.Println("Error: ", err)
			http.Error(w, err.Error(), 400)
			return
		}
		var startTime time.Time
		if result.StartTime != nil {
			i, f := math.Modf(*result.StartTime)
			startTime = time.Unix(int64(i), int64(f*1e9))
		}
		h.app.setMidiPlaybackScheduler(result.Enabled, startTime, result.LoopEnabled, time.Duration(result.LoopInterval*1e9)*time.Nanosecond)
	}

	h.app.MidiPlaybackGoro.Submit(h.app.ctx, func(context.Context) (interface{}, error) {
		var (
			startTime    time.Time
			loopInterval time.Duration
		)
		result.Enabled, startTime, result.LoopEnabled, loopInterval = h.app.getMidiPlaybackScheduler()
		if !startTime.IsZero() {
			startTime = startTime.UTC()
			result.StartTime = new(float64)
			*result.StartTime = float64(startTime.Unix()) + float64(startTime.Nanosecond())*1e-9
		}
		result.LoopInterval = float64(loopInterval/time.Nanosecond) * 1e-9
		return nil, nil
	})
	writeJSON(w, result)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	stream, err := json.Marshal(v)
	if err != nil {
		log.Println("Error: ", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(stream)
}

func (h *basicAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if user, pass, _ := r.BasicAuth(); user != h.Username || pass != h.Password {
		w.Header().Set("WWW-Authenticate", "Basic")
		http.Error(w, "Unauthorized", 401)
		return
	}
	h.Handler.ServeHTTP(w, r)
}
