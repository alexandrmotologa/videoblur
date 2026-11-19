package vision

import (
	"math"

	"github.com/alexandrmotologa/videoblur/internal/config"
	"github.com/alexandrmotologa/videoblur/internal/domain"
)

// ApplyRedaction applies the chosen redaction mode (Gaussian, Mosaic, Blackout)
// to the target region within an RGB24 frame buffer.
func ApplyRedaction(rgb []byte, width, height int, rect domain.Rect, mode config.RedactionMode, sigma float64, blockSize int) {
	// Clamp rect to frame bounds
	x1 := max(0, rect.X)
	y1 := max(0, rect.Y)
	x2 := min(width, rect.X+rect.W)
	y2 := min(height, rect.Y+rect.H)

	if x2 <= x1 || y2 <= y1 {
		return
	}

	subW := x2 - x1
	subH := y2 - y1

	switch mode {
	case config.ModeBlackout:
		applyBlackout(rgb, width, x1, y1, subW, subH)
	case config.ModeMosaic:
		if blockSize <= 0 {
			blockSize = 20
		}
		applyMosaic(rgb, width, x1, y1, subW, subH, blockSize)
	case config.ModeGaussianBlur:
		fallthrough
	default:
		if sigma <= 0 {
			sigma = 16.0
		}
		applyGaussianBlur(rgb, width, height, x1, y1, subW, subH, sigma)
	}
}

// applyBlackout fills the bounding region with pure black pixels (0, 0, 0).
func applyBlackout(rgb []byte, frameWidth, x0, y0, w, h int) {
	for y := y0; y < y0+h; y++ {
		rowOffset := y * frameWidth * 3
		for x := x0; x < x0+w; x++ {
			idx := rowOffset + x*3
			rgb[idx] = 0
			rgb[idx+1] = 0
			rgb[idx+2] = 0
		}
	}
}

// applyMosaic averages colors within block_size x block_size pixel cells.
func applyMosaic(rgb []byte, frameWidth, x0, y0, w, h, blockSize int) {
	for by := y0; by < y0+h; by += blockSize {
		bh := min(blockSize, y0+h-by)
		for bx := x0; bx < x0+w; bx += blockSize {
			bw := min(blockSize, x0+w-bx)

			// Calculate average color in block
			var sumR, sumG, sumB int64
			count := int64(bw * bh)
			for y := by; y < by+bh; y++ {
				rowOffset := y * frameWidth * 3
				for x := bx; x < bx+bw; x++ {
					idx := rowOffset + x*3
					sumR += int64(rgb[idx])
					sumG += int64(rgb[idx+1])
					sumB += int64(rgb[idx+2])
				}
			}

			avgR := byte(sumR / count)
			avgG := byte(sumG / count)
			avgB := byte(sumB / count)

			// Paint averaged color over the entire cell
			for y := by; y < by+bh; y++ {
				rowOffset := y * frameWidth * 3
				for x := bx; x < bx+bw; x++ {
					idx := rowOffset + x*3
					rgb[idx] = avgR
					rgb[idx+1] = avgG
					rgb[idx+2] = avgB
				}
			}
		}
	}
}

// applyGaussianBlur performs a fast separable 2-pass Gaussian blur on the target subregion.
func applyGaussianBlur(rgb []byte, frameWidth, frameHeight, x0, y0, w, h int, sigma float64) {
	radius := int(math.Ceil(sigma * 2.5))
	if radius < 2 {
		radius = 2
	}
	if radius > 45 {
		radius = 45 // upper bound to balance blur radius and processing speed
	}

	kernel := buildGaussianKernel(radius, sigma)

	// Extract subregion into intermediate buffer
	subRGB := make([]byte, w*h*3)
	for y := 0; y < h; y++ {
		srcRow := (y0 + y) * frameWidth * 3
		dstRow := y * w * 3
		copy(subRGB[dstRow:dstRow+w*3], rgb[srcRow+x0*3:srcRow+(x0+w)*3])
	}

	// Horizontal blur pass into temp buffer
	tempRGB := make([]byte, len(subRGB))
	for y := 0; y < h; y++ {
		rowOffset := y * w * 3
		for x := 0; x < w; x++ {
			var rAcc, gAcc, bAcc float64
			for k := -radius; k <= radius; k++ {
				sampleX := clamp(x+k, 0, w-1)
				idx := rowOffset + sampleX*3
				weight := kernel[k+radius]
				rAcc += float64(subRGB[idx]) * weight
				gAcc += float64(subRGB[idx+1]) * weight
				bAcc += float64(subRGB[idx+2]) * weight
			}
			outIdx := rowOffset + x*3
			tempRGB[outIdx] = byte(math.Min(255, math.Max(0, rAcc)))
			tempRGB[outIdx+1] = byte(math.Min(255, math.Max(0, gAcc)))
			tempRGB[outIdx+2] = byte(math.Min(255, math.Max(0, bAcc)))
		}
	}

	// Vertical blur pass writing directly back to the original frame
	for y := 0; y < h; y++ {
		dstRow := (y0 + y) * frameWidth * 3
		for x := 0; x < w; x++ {
			var rAcc, gAcc, bAcc float64
			for k := -radius; k <= radius; k++ {
				sampleY := clamp(y+k, 0, h-1)
				idx := sampleY*w*3 + x*3
				weight := kernel[k+radius]
				rAcc += float64(tempRGB[idx]) * weight
				gAcc += float64(tempRGB[idx+1]) * weight
				bAcc += float64(tempRGB[idx+2]) * weight
			}
			outIdx := dstRow + (x0+x)*3
			rgb[outIdx] = byte(math.Min(255, math.Max(0, rAcc)))
			rgb[outIdx+1] = byte(math.Min(255, math.Max(0, gAcc)))
			rgb[outIdx+2] = byte(math.Min(255, math.Max(0, bAcc)))
		}
	}
}

func buildGaussianKernel(radius int, sigma float64) []float64 {
	size := 2*radius + 1
	kernel := make([]float64, size)
	twoSigmaSq := 2.0 * sigma * sigma
	var sum float64

	for i := -radius; i <= radius; i++ {
		val := math.Exp(-float64(i*i) / twoSigmaSq)
		kernel[i+radius] = val
		sum += val
	}

	// Normalize kernel weights so they sum to 1.0
	for i := range kernel {
		kernel[i] /= sum
	}

	return kernel
}

func clamp(val, low, high int) int {
	if val < low {
		return low
	}
	if val > high {
		return high
	}
	return val
}
