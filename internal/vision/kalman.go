package vision

import (
	"math"

	"github.com/alexandrmotologa/videoblur/internal/domain"
)

// KalmanBoxFilter implements a 7-state linear Kalman filter for bounding box tracking:
// State: [u, v, s, r, u_dot, v_dot, s_dot]^T
// u: center x, v: center y, s: scale (area), r: aspect ratio (w/h)
type KalmanBoxFilter struct {
	x [7]float64    // State estimate
	p [7][7]float64 // Estimate covariance matrix
	q [7][7]float64 // Process noise covariance
	r [4][4]float64 // Measurement noise covariance
}

// NewKalmanBoxFilter initializes a filter from a bounding box measurement.
func NewKalmanBoxFilter(box domain.Rect) *KalmanBoxFilter {
	kf := &KalmanBoxFilter{}
	cx, cy := box.Center()
	s := box.Area()
	if s <= 0 {
		s = 1.0
	}
	r := box.AspectRatio()
	if r <= 0 {
		r = 1.0
	}

	kf.x[0] = cx
	kf.x[1] = cy
	kf.x[2] = s
	kf.x[3] = r
	kf.x[4] = 0
	kf.x[5] = 0
	kf.x[6] = 0

	// Initialize state covariance P
	for i := 0; i < 7; i++ {
		for j := 0; j < 7; j++ {
			if i == j {
				if i < 4 {
					kf.p[i][j] = 10.0
				} else {
					kf.p[i][j] = 1000.0 // high uncertainty for velocities initially
				}
			}
		}
	}

	// Initialize process noise Q
	for i := 0; i < 7; i++ {
		if i < 4 {
			kf.q[i][i] = 1.0
		} else {
			kf.q[i][i] = 0.01
		}
	}

	// Initialize measurement noise R
	for i := 0; i < 4; i++ {
		kf.r[i][i] = 1.0
	}
	kf.r[2][2] = 10.0 // area measurement variance

	return kf
}

// Predict steps the state forward in time (dt = 1 frame).
func (kf *KalmanBoxFilter) Predict() domain.Rect {
	// F matrix: x_next = F * x
	// u' = u + u_dot
	// v' = v + v_dot
	// s' = s + s_dot
	// r' = r
	// u_dot' = u_dot
	// v_dot' = v_dot
	// s_dot' = s_dot
	kf.x[0] += kf.x[4]
	kf.x[1] += kf.x[5]
	kf.x[2] += kf.x[6]
	if kf.x[2] < 1.0 {
		kf.x[2] = 1.0
	}

	// Covariance update: P = F * P * F^T + Q
	// We compute the algebraic expansion directly for optimal execution speed:
	p := &kf.p
	p[0][0] += 2*p[0][4] + p[4][4] + kf.q[0][0]
	p[0][4] += p[4][4]
	p[4][0] = p[0][4]
	p[4][4] += kf.q[4][4]

	p[1][1] += 2*p[1][5] + p[5][5] + kf.q[1][1]
	p[1][5] += p[5][5]
	p[5][1] = p[1][5]
	p[5][5] += kf.q[5][5]

	p[2][2] += 2*p[2][6] + p[6][6] + kf.q[2][2]
	p[2][6] += p[6][6]
	p[6][2] = p[2][6]
	p[6][6] += kf.q[6][6]

	p[3][3] += kf.q[3][3]

	return kf.StateToRect()
}

// Update incorporates a new bounding box detection measurement z = [u, v, s, r]^T.
func (kf *KalmanBoxFilter) Update(box domain.Rect) {
	cx, cy := box.Center()
	s := box.Area()
	if s <= 0 {
		s = 1.0
	}
	r := box.AspectRatio()
	if r <= 0 {
		r = 1.0
	}

	z := [4]float64{cx, cy, s, r}

	// Innovation: y = z - H * x
	// Since H maps [u, v, s, r, ...] to [u, v, s, r], H*x is x[0..3]
	y := [4]float64{
		z[0] - kf.x[0],
		z[1] - kf.x[1],
		z[2] - kf.x[2],
		z[3] - kf.x[3],
	}

	// Innovation covariance: S = H * P * H^T + R
	// Since H is identity for top-left 4x4, S is simply P[0..3][0..3] + R
	for i := 0; i < 4; i++ {
		s := kf.p[i][i] + kf.r[i][i]
		if s <= 1e-6 {
			s = 1e-6
		}
		// Kalman gain K_i = P_i / S_i (for diagonal approximation)
		kGain := kf.p[i][i] / s
		kf.x[i] += kGain * y[i]
		kf.p[i][i] *= (1.0 - kGain)

		// Cross-term velocity updates
		if i < 3 {
			vIdx := i + 4
			kVel := kf.p[vIdx][i] / s
			kf.x[vIdx] += kVel * y[i]
			kf.p[vIdx][vIdx] -= kVel * kf.p[i][vIdx]
		}
	}

	if kf.x[2] < 1.0 {
		kf.x[2] = 1.0
	}
	if kf.x[3] <= 0 {
		kf.x[3] = 1.0
	}
}

// StateToRect converts internal state [u, v, s, r] into a pixel Rect [x, y, w, h].
func (kf *KalmanBoxFilter) StateToRect() domain.Rect {
	cx := kf.x[0]
	cy := kf.x[1]
	s := kf.x[2]
	r := kf.x[3]

	if s <= 0 {
		s = 1.0
	}
	if r <= 0 {
		r = 1.0
	}

	// s = w * h, r = w / h => w^2 = s * r, h^2 = s / r
	w := math.Sqrt(s * r)
	h := math.Sqrt(s / r)

	x := int(math.Round(cx - w/2.0))
	y := int(math.Round(cy - h/2.0))
	width := int(math.Round(w))
	height := int(math.Round(h))

	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	return domain.Rect{
		X: x,
		Y: y,
		W: width,
		H: height,
	}
}
