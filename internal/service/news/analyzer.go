package news

import (
	"context"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/ports"
)

// Analyzer implements mathematical logic for News processing
type Analyzer struct {
	ai        ports.AIAnalyzer
	prefsRepo ports.PrefsRepository
}

// NewAnalyzer creates a new Analyzer instance
func NewAnalyzer(ai ports.AIAnalyzer, prefsRepo ports.PrefsRepository) *Analyzer {
	return &Analyzer{
		ai:        ai,
		prefsRepo: prefsRepo,
	}
}

// (Surprise Factor Math implementations can go here)
// e.g. CalcSurpriseScore(actual, forecast, stdDev) floats ...
