package causal

import (
	"errors"
	"fmt"
	"math"
)

// Matrix is a dense row-major matrix.
type Matrix struct {
	rows, cols int
	data       []float64
}

func NewMatrix(r, c int) *Matrix {
	return &Matrix{rows: r, cols: c, data: make([]float64, r*c)}
}

func (m *Matrix) Set(r, c int, v float64) { m.data[r*m.cols+c] = v }
func (m *Matrix) Get(r, c int) float64    { return m.data[r*m.cols+c] }

// Identity returns an n×n identity matrix.
func Identity(n int) *Matrix {
	m := NewMatrix(n, n)
	for i := 0; i < n; i++ {
		m.Set(i, i, 1)
	}
	return m
}

// Clone returns a deep copy.
func (m *Matrix) Clone() *Matrix {
	c := NewMatrix(m.rows, m.cols)
	copy(c.data, m.data)
	return c
}

// Multiply returns m × b.
func (m *Matrix) Multiply(b *Matrix) (*Matrix, error) {
	if m.cols != b.rows {
		return nil, fmt.Errorf("dimension mismatch: (%d,%d) × (%d,%d)", m.rows, m.cols, b.rows, b.cols)
	}
	result := NewMatrix(m.rows, b.cols)
	for i := 0; i < m.rows; i++ {
		for k := 0; k < m.cols; k++ {
			if m.Get(i, k) == 0 {
				continue
			}
			for j := 0; j < b.cols; j++ {
				result.data[i*b.cols+j] += m.Get(i, k) * b.Get(k, j)
			}
		}
	}
	return result, nil
}

// Transpose returns mᵀ.
func (m *Matrix) Transpose() *Matrix {
	t := NewMatrix(m.cols, m.rows)
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			t.Set(j, i, m.Get(i, j))
		}
	}
	return t
}

// Inverse computes m⁻¹ via Gauss-Jordan with partial pivoting.
// Returns error if matrix is singular or non-square.
func Inverse(m *Matrix) (*Matrix, error) {
	if m.rows != m.cols {
		return nil, errors.New("matrix must be square")
	}
	n := m.rows
	// augment [m | I]
	aug := NewMatrix(n, 2*n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			aug.Set(i, j, m.Get(i, j))
		}
		aug.Set(i, n+i, 1)
	}

	for col := 0; col < n; col++ {
		// partial pivot
		maxRow, maxVal := col, math.Abs(aug.Get(col, col))
		for row := col + 1; row < n; row++ {
			if v := math.Abs(aug.Get(row, col)); v > maxVal {
				maxVal, maxRow = v, row
			}
		}
		if maxVal < 1e-12 {
			return nil, errors.New("matrix is singular or nearly singular")
		}
		// swap rows
		if maxRow != col {
			for j := 0; j < 2*n; j++ {
				aug.data[col*2*n+j], aug.data[maxRow*2*n+j] = aug.data[maxRow*2*n+j], aug.data[col*2*n+j]
			}
		}
		// scale pivot row
		pivot := aug.Get(col, col)
		for j := 0; j < 2*n; j++ {
			aug.data[col*2*n+j] /= pivot
		}
		// eliminate column
		for row := 0; row < n; row++ {
			if row == col {
				continue
			}
			factor := aug.Get(row, col)
			if factor == 0 {
				continue
			}
			for j := 0; j < 2*n; j++ {
				aug.data[row*2*n+j] -= factor * aug.data[col*2*n+j]
			}
		}
	}

	inv := NewMatrix(n, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			inv.Set(i, j, aug.Get(i, n+j))
		}
	}
	return inv, nil
}

// OLS solves β = (XᵀX)⁻¹ Xᵀy. The caller must NOT prepend an intercept column;
// OLS prepends a column of ones automatically.
// Returns coefficients [intercept, β₁, β₂, ...].
func OLS(y []float64, X *Matrix) ([]float64, error) {
	n := len(y)
	if X.rows != n {
		return nil, fmt.Errorf("X has %d rows but y has %d elements", X.rows, n)
	}
	p := X.cols + 1 // +1 for intercept
	// build design matrix with leading ones column
	D := NewMatrix(n, p)
	for i := 0; i < n; i++ {
		D.Set(i, 0, 1)
		for j := 0; j < X.cols; j++ {
			D.Set(i, j+1, X.Get(i, j))
		}
	}
	Dt := D.Transpose()
	DtD, err := Dt.Multiply(D)
	if err != nil {
		return nil, err
	}
	inv, err := Inverse(DtD)
	if err != nil {
		return nil, fmt.Errorf("OLS: XᵀX singular: %w", err)
	}
	// Xᵀy
	Dty := make([]float64, p)
	for j := 0; j < p; j++ {
		for i := 0; i < n; i++ {
			Dty[j] += D.Get(i, j) * y[i]
		}
	}
	beta := make([]float64, p)
	for i := 0; i < p; i++ {
		for j := 0; j < p; j++ {
			beta[i] += inv.Get(i, j) * Dty[j]
		}
	}
	return beta, nil
}
