// +build js,wasm

package audio

import "syscall/js"

// exposed from web/audio.js
var play js.Value

func init() {
	global := js.Global()
	play = global.Get("play") // (id, when) exported by JS
}

func PlaySample(id string, when float64) {
	play.Invoke(id, when)
}

