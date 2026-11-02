// SPDX-License-Identifier: MIT
package asset

import (
	"bytes"
	"encoding/json"
)

type Frame struct {
	Index int
	Time  int
}

func (frame *Frame) UnmarshalJSON(data []byte) error {
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) {
		var value struct {
			Index int `json:"index"`
			Time  int `json:"time"`
		}
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*frame = Frame{Index: value.Index, Time: value.Time}
		return nil
	}
	return json.Unmarshal(data, &frame.Index)
}

type Animation struct {
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	FrameTime   int     `json:"frametime"`
	Frames      []Frame `json:"frames"`
	Interpolate bool    `json:"interpolate"`
}
type TextureMetadata struct {
	Animation *Animation `json:"animation"`
}
