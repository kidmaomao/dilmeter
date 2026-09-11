package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// normalizeWaveToPCM16 converts the WAV variants commonly produced by audio
// editors and TTS tools (8/16/24/32-bit PCM, 32/64-bit float and extensible
// PCM/float) to a canonical 16-bit PCM RIFF file. Windows MCI is much more
// reliable with this format, including while the UI is minimized.
func normalizeWaveToPCM16(input []byte) ([]byte, error) {
	if len(input) < 12 || string(input[:4]) != "RIFF" || string(input[8:12]) != "WAVE" {
		return nil, errors.New("missing RIFF/WAVE header")
	}
	var formatChunk, sampleData []byte
	for offset := 12; offset+8 <= len(input); {
		size := int(binary.LittleEndian.Uint32(input[offset+4 : offset+8]))
		start := offset + 8
		end := start + size
		if size < 0 || end < start || end > len(input) {
			return nil, errors.New("truncated WAV chunk")
		}
		switch string(input[offset : offset+4]) {
		case "fmt ":
			formatChunk = input[start:end]
		case "data":
			sampleData = input[start:end]
		}
		offset = end + size%2
	}
	if len(formatChunk) < 16 || sampleData == nil {
		return nil, errors.New("missing fmt or data chunk")
	}

	formatTag := binary.LittleEndian.Uint16(formatChunk[0:2])
	channels := binary.LittleEndian.Uint16(formatChunk[2:4])
	sampleRate := binary.LittleEndian.Uint32(formatChunk[4:8])
	blockAlign := binary.LittleEndian.Uint16(formatChunk[12:14])
	bitsPerSample := binary.LittleEndian.Uint16(formatChunk[14:16])
	if formatTag == 0xfffe {
		if len(formatChunk) < 40 {
			return nil, errors.New("truncated extensible format")
		}
		// The first two bytes of KSDATAFORMAT_SUBTYPE_PCM / IEEE_FLOAT carry
		// the original WAVE format tag.
		formatTag = binary.LittleEndian.Uint16(formatChunk[24:26])
	}
	if channels == 0 || channels > 8 || sampleRate < 1000 || sampleRate > 384000 {
		return nil, errors.New("unsupported channel count or sample rate")
	}
	bytesPerSample := int((bitsPerSample + 7) / 8)
	if bytesPerSample == 0 || int(blockAlign) < int(channels)*bytesPerSample {
		return nil, errors.New("invalid sample alignment")
	}
	if formatTag == 1 && bitsPerSample != 8 && bitsPerSample != 16 && bitsPerSample != 24 && bitsPerSample != 32 {
		return nil, fmt.Errorf("unsupported PCM depth %d", bitsPerSample)
	}
	if formatTag == 3 && bitsPerSample != 32 && bitsPerSample != 64 {
		return nil, fmt.Errorf("unsupported float depth %d", bitsPerSample)
	}
	if formatTag != 1 && formatTag != 3 {
		return nil, fmt.Errorf("compressed WAV format %d is not supported", formatTag)
	}

	frameSize := int(blockAlign)
	frameCount := len(sampleData) / frameSize
	outputData := make([]byte, frameCount*int(channels)*2)
	outputOffset := 0
	for frame := 0; frame < frameCount; frame++ {
		frameOffset := frame * frameSize
		for channel := 0; channel < int(channels); channel++ {
			sampleOffset := frameOffset + channel*bytesPerSample
			value, err := decodeWaveSample(sampleData[sampleOffset:sampleOffset+bytesPerSample], formatTag, bitsPerSample)
			if err != nil {
				return nil, err
			}
			binary.LittleEndian.PutUint16(outputData[outputOffset:outputOffset+2], uint16(value))
			outputOffset += 2
		}
	}

	output := make([]byte, 44+len(outputData))
	copy(output[0:4], "RIFF")
	binary.LittleEndian.PutUint32(output[4:8], uint32(len(output)-8))
	copy(output[8:12], "WAVE")
	copy(output[12:16], "fmt ")
	binary.LittleEndian.PutUint32(output[16:20], 16)
	binary.LittleEndian.PutUint16(output[20:22], 1)
	binary.LittleEndian.PutUint16(output[22:24], channels)
	binary.LittleEndian.PutUint32(output[24:28], sampleRate)
	outputBlockAlign := channels * 2
	binary.LittleEndian.PutUint32(output[28:32], sampleRate*uint32(outputBlockAlign))
	binary.LittleEndian.PutUint16(output[32:34], outputBlockAlign)
	binary.LittleEndian.PutUint16(output[34:36], 16)
	copy(output[36:40], "data")
	binary.LittleEndian.PutUint32(output[40:44], uint32(len(outputData)))
	copy(output[44:], outputData)
	return output, nil
}

func decodeWaveSample(sample []byte, formatTag, bitsPerSample uint16) (int16, error) {
	if formatTag == 1 {
		switch bitsPerSample {
		case 8:
			return int16((int(sample[0]) - 128) << 8), nil
		case 16:
			return int16(binary.LittleEndian.Uint16(sample)), nil
		case 24:
			value := int32(sample[0]) | int32(sample[1])<<8 | int32(sample[2])<<16
			if value&0x800000 != 0 {
				value |= ^int32(0xffffff)
			}
			return int16(value >> 8), nil
		case 32:
			return int16(int32(binary.LittleEndian.Uint32(sample)) >> 16), nil
		}
	}
	if formatTag == 3 {
		var value float64
		if bitsPerSample == 32 {
			value = float64(math.Float32frombits(binary.LittleEndian.Uint32(sample)))
		} else {
			value = math.Float64frombits(binary.LittleEndian.Uint64(sample))
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			value = 0
		}
		value = math.Max(-1, math.Min(1, value))
		if value <= -1 {
			return -32768, nil
		}
		return int16(math.Round(value * 32767)), nil
	}
	return 0, errors.New("unsupported WAV sample")
}
