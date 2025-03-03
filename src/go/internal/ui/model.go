package ui

type GridModel struct {
	Rows []*DrumRow           // drum-machine representation
}

func NewModel() *GridModel {
	// single-row prototype; name “H” (Hi-Hat)
	rows := []*DrumRow{{Name: "H"}}
	return &GridModel{Rows: rows}
}

func (m *GridModel) SetLength(newLen int) {
	if newLen < 4 { newLen = 4 }
	if newLen > 64 { newLen = 64 }
	for _, r := range m.Rows {
		if len(r.Steps) == newLen { continue }
		if len(r.Steps) < newLen {
			padding := make([]bool, newLen-len(r.Steps))
			r.Steps = append(r.Steps, padding...)
		} else {
			r.Steps = r.Steps[:newLen]
		}
	}
}

func (m *GridModel) ensureLen(i int) {
	for _, r := range m.Rows {
		if i >= len(r.Steps) {
			newSteps := make([]bool, i+1)
			copy(newSteps, r.Steps)
			r.Steps = newSteps
		}
	}
}

// Toggle vertex (i,j)
func (m *GridModel) Toggle(i, j int) {
	if j < 0 || j >= len(m.Rows) {
		return
	}
	m.ensureLen(i)
	r := m.Rows[j]
	r.Steps[i] = !r.Steps[i]
}

