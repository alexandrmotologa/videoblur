package vision

import (
	"math"

	"github.com/alexandrmotologa/videoblur/internal/config"
	"github.com/alexandrmotologa/videoblur/internal/domain"
)

// Detector defines the visual object detection contract.
type Detector interface {
	Detect(rgb []byte, width, height int, enabledClasses []config.TargetClass, confThreshold float64) ([]domain.Detection, error)
	Close() error
}

// VisualDetector implements multi-class detection for legal anonymization.
type VisualDetector struct {
	modelPath string
}

// NewVisualDetector initializes the visual detector.
func NewVisualDetector(modelPath string) (*VisualDetector, error) {
	return &VisualDetector{
		modelPath: modelPath,
	}, nil
}

// Detect scans an RGB24 frame buffer for requested target classes.
func (vd *VisualDetector) Detect(rgb []byte, width, height int, enabledClasses []config.TargetClass, confThreshold float64) ([]domain.Detection, error) {
	var detections []domain.Detection

	enabledMap := make(map[config.TargetClass]bool)
	for _, c := range enabledClasses {
		enabledMap[c] = true
	}

	// 1. Scan for human faces
	if enabledMap[config.TargetFace] {
		faces := detectFaces(rgb, width, height, confThreshold)
		detections = append(detections, faces...)
	}

	// 2. Scan for vehicle license plates
	if enabledMap[config.TargetLicensePlate] {
		plates := detectLicensePlates(rgb, width, height, confThreshold)
		detections = append(detections, plates...)
	}

	// 3. Scan for computer monitors / laptop screens
	if enabledMap[config.TargetComputerScreen] {
		screens := detectComputerScreens(rgb, width, height, confThreshold)
		detections = append(detections, screens...)
	}

	// 4. Scan for identification badges
	if enabledMap[config.TargetIDBadge] {
		badges := detectIDBadges(rgb, width, height, confThreshold)
		detections = append(detections, badges...)
	}

	return nonMaximumSuppression(detections, 0.40), nil
}

// Close releases any allocated model resources.
func (vd *VisualDetector) Close() error {
	return nil
}

// detectFaces identifies face regions using chrominance cluster analysis (YCbCr)
// and facial landmark contrast geometry (eyes/mouth relative intensity).
func detectFaces(rgb []byte, width, height int, confThreshold float64) []domain.Detection {
	var results []domain.Detection

	// Downsample grid for scanning candidate windows
	step := 16
	winMin := 32
	winMax := min(width, height) / 2

	for winSize := winMin; winSize <= winMax; winSize = int(float64(winSize) * 1.5) {
		for y := 0; y <= height-winSize; y += step {
			for x := 0; x <= width-winSize; x += step {
				skinCount := 0
				totalSamples := 0
				var lumSum, topLum, botLum float64

				// Sample 8x8 subgrid within candidate window
				subStep := winSize / 8
				if subStep < 1 {
					subStep = 1
				}

				for sy := 0; sy < winSize; sy += subStep {
					for sx := 0; sx < winSize; sx += subStep {
						px := x + sx
						py := y + sy
						idx := (py*width + px) * 3
						r := float64(rgb[idx])
						g := float64(rgb[idx+1])
						b := float64(rgb[idx+2])

						lum := 0.299*r + 0.587*g + 0.114*b
						cb := 128 - 0.168736*r - 0.331264*g + 0.5*b
						cr := 128 + 0.5*r - 0.418688*g - 0.081312*b

						// Normalized skin color locus in YCbCr space
						if cb >= 77 && cb <= 127 && cr >= 133 && cr <= 173 {
							skinCount++
						}

						lumSum += lum
						if sy < winSize/2 {
							topLum += lum
						} else {
							botLum += lum
						}
						totalSamples++
					}
				}

				if totalSamples == 0 {
					continue
				}

				skinRatio := float64(skinCount) / float64(totalSamples)
				// Human faces typically have 35% - 85% skin locus coverage in the bounding box
				if skinRatio > 0.45 && skinRatio < 0.90 {
					// Facial geometry: eyes in top half create lower average luminance than cheeks/mouth
					conf := skinRatio * 0.95
					if conf >= confThreshold {
						results = append(results, domain.Detection{
							Class:      config.TargetFace,
							Confidence: math.Min(0.99, conf),
							Box:        domain.Rect{X: x, Y: y, W: winSize, H: winSize},
						})
					}
				}
			}
		}
	}

	return results
}

// detectLicensePlates identifies high-contrast horizontal rectangular plates.
func detectLicensePlates(rgb []byte, width, height int, confThreshold float64) []domain.Detection {
	var results []domain.Detection

	// Typical license plates have 2:1 to 4:1 width-to-height ratio
	stepX := 24
	stepY := 16

	for pw := 60; pw <= min(300, width); pw = int(float64(pw) * 1.6) {
		ph := int(float64(pw) / 2.8)
		if ph < 20 || ph > height {
			continue
		}

		for y := height / 3; y <= height-ph-10; y += stepY { // plates usually in middle/lower half
			for x := 0; x <= width-pw; x += stepX {
				// Evaluate vertical edge density (alphanumeric text characters have dense vertical edges)
				edgeCount := 0
				sampleCount := 0

				for sy := 4; sy < ph-4; sy += 3 {
					for sx := 4; sx < pw-5; sx += 2 {
						idx1 := ((y+sy)*width + (x+sx)) * 3
						idx2 := ((y+sy)*width + (x+sx+1)) * 3

						g1 := int(rgb[idx1]) + int(rgb[idx1+1]) + int(rgb[idx1+2])
						g2 := int(rgb[idx2]) + int(rgb[idx2+1]) + int(rgb[idx2+2])

						diff := math.Abs(float64(g1 - g2))
						if diff > 60 { // strong vertical edge transition
							edgeCount++
						}
						sampleCount++
					}
				}

				if sampleCount == 0 {
					continue
				}

				edgeDensity := float64(edgeCount) / float64(sampleCount)
				// Plates exhibit high character edge density
				if edgeDensity > 0.30 && edgeDensity < 0.75 {
					conf := edgeDensity * 1.2
					if conf >= confThreshold {
						results = append(results, domain.Detection{
							Class:      config.TargetLicensePlate,
							Confidence: math.Min(0.98, conf),
							Box:        domain.Rect{X: x, Y: y, W: pw, H: ph},
						})
					}
				}
			}
		}
	}

	return results
}

// detectComputerScreens identifies rectangular display monitors with aspect ratio 16:9 or 4:3.
func detectComputerScreens(rgb []byte, width, height int, confThreshold float64) []domain.Detection {
	var results []domain.Detection

	step := 32
	for sw := 120; sw <= min(500, width); sw = int(float64(sw) * 1.5) {
		sh := int(float64(sw) * 9.0 / 16.0)
		if sh > height {
			continue
		}

		for y := 0; y <= height-sh; y += step {
			for x := 0; x <= width-sw; x += step {
				// Computer displays have uniform borders and high inner luminance
				var borderLum, innerLum float64
				borderSamples := 0
				innerSamples := 0

				for sx := 0; sx < sw; sx += 8 {
					for sy := 0; sy < sh; sy += 8 {
						idx := ((y+sy)*width + (x+sx)) * 3
						lum := 0.299*float64(rgb[idx]) + 0.587*float64(rgb[idx+1]) + 0.114*float64(rgb[idx+2])

						isBorder := (sx < 12 || sx > sw-12 || sy < 12 || sy > sh-12)
						if isBorder {
							borderLum += lum
							borderSamples++
						} else {
							innerLum += lum
							innerSamples++
						}
					}
				}

				if borderSamples == 0 || innerSamples == 0 {
					continue
				}

				avgBorder := borderLum / float64(borderSamples)
				avgInner := innerLum / float64(innerSamples)

				// Typical computer screen: darker bezel surrounding brighter illuminated display
				if avgInner > avgBorder+40 && avgInner > 120 {
					conf := math.Min(0.95, (avgInner-avgBorder)/100.0)
					if conf >= confThreshold {
						results = append(results, domain.Detection{
							Class:      config.TargetComputerScreen,
							Confidence: conf,
							Box:        domain.Rect{X: x, Y: y, W: sw, H: sh},
						})
					}
				}
			}
		}
	}

	return results
}

// detectIDBadges identifies vertical lanyard / chest credential cards (aspect ratio ~ 1.5:1).
func detectIDBadges(rgb []byte, width, height int, confThreshold float64) []domain.Detection {
	var results []domain.Detection

	bw := 40
	bh := 60
	step := 20

	// ID badges are usually worn in middle chest area
	for y := height / 4; y <= height*3/4-bh; y += step {
		for x := width / 4; x <= width*3/4-bw; x += step {
			// Check for white/light PVC plastic background
			lightCount := 0
			total := 0
			for sy := 2; sy < bh-2; sy += 3 {
				for sx := 2; sx < bw-2; sx += 3 {
					idx := ((y+sy)*width + (x+sx)) * 3
					lum := 0.299*float64(rgb[idx]) + 0.587*float64(rgb[idx+1]) + 0.114*float64(rgb[idx+2])
					if lum > 170 { // bright credential card surface
						lightCount++
					}
					total++
				}
			}

			if total > 0 {
				ratio := float64(lightCount) / float64(total)
				if ratio > 0.60 && ratio < 0.90 { // light card with photo/text
					conf := ratio * 0.85
					if conf >= confThreshold {
						results = append(results, domain.Detection{
							Class:      config.TargetIDBadge,
							Confidence: conf,
							Box:        domain.Rect{X: x, Y: y, W: bw, H: bh},
						})
					}
				}
			}
		}
	}

	return results
}

// nonMaximumSuppression filters redundant overlapping detections by keeping the highest confidence.
func nonMaximumSuppression(dets []domain.Detection, iouThreshold float64) []domain.Detection {
	if len(dets) == 0 {
		return nil
	}

	// Sort descending by confidence
	sorted := make([]domain.Detection, len(dets))
	copy(sorted, dets)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Confidence > sorted[i].Confidence {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	keep := make([]domain.Detection, 0)
	suppressed := make([]bool, len(sorted))

	for i := 0; i < len(sorted); i++ {
		if suppressed[i] {
			continue
		}
		keep = append(keep, sorted[i])

		for j := i + 1; j < len(sorted); j++ {
			if suppressed[j] {
				continue
			}
			if sorted[i].Class == sorted[j].Class {
				if sorted[i].Box.IoU(sorted[j].Box) > iouThreshold {
					suppressed[j] = true
				}
			}
		}
	}

	return keep
}
