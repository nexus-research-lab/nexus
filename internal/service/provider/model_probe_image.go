// INPUT: Cryptographic randomness, without user attachments or model-name assumptions.
// OUTPUT: A small inline PNG challenge and an answer only encoded in its pixels.
// POS: Vision probe fixture generation; no filesystem, remote image host or user data.
package provider

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

func newVisionProbeImage() (data, answer string, err error) {
	var random [9]byte
	if _, err = rand.Read(random[:]); err != nil {
		return "", "", err
	}
	palette := []color.RGBA{{220, 25, 25, 255}, {20, 160, 40, 255}, {25, 50, 220, 255}, {245, 215, 15, 255}}
	canvas := image.NewRGBA(image.Rect(0, 0, 192, 192))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	for i, value := range random {
		index := int(value % 4)
		x, y := (i%3)*64, (i/3)*64
		draw.Draw(canvas, image.Rect(x+4, y+4, x+60, y+60), image.NewUniform(palette[index]), image.Point{}, draw.Src)
		answer += string(rune('1' + index))
	}
	var encoded bytes.Buffer
	if err = png.Encode(&encoded, canvas); err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(encoded.Bytes()), answer, nil
}
