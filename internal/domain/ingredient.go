package domain

import (
	"math"
	"strings"
)

type FormulaIngredient struct {
	Sequence         int     `json:"sequence"`
	Name             string  `json:"name"`
	Unit             string  `json:"unit"`
	QuantityPerLiter float64 `json:"quantity_per_liter"`
}

type RunIngredient struct {
	Sequence         int     `json:"sequence"`
	Name             string  `json:"name"`
	Unit             string  `json:"unit"`
	QuantityPerLiter float64 `json:"quantity_per_liter"`
	PlannedQuantity  float64 `json:"planned_quantity"`
}

func ValidateFormulaIngredients(ingredients []FormulaIngredient, required bool) error {
	if required && len(ingredients) == 0 {
		return FieldError(CodeInvalidInput, "ingredients", "must contain at least one entry")
	}
	if len(ingredients) > 64 {
		return FieldError(CodeInvalidInput, "ingredients", "must contain at most 64 entries")
	}
	for index, ingredient := range ingredients {
		expected := index + 1
		if ingredient.Sequence != expected {
			return FieldError(CodeInvalidInput, "ingredients",
				"sequence values must be contiguous and begin at 1")
		}
		if strings.TrimSpace(ingredient.Name) == "" {
			return FieldError(CodeInvalidInput, "ingredients.name", "is required")
		}
		if len([]rune(ingredient.Name)) > 120 {
			return FieldError(CodeInvalidInput, "ingredients.name",
				"must not exceed 120 characters")
		}
		if strings.TrimSpace(ingredient.Unit) == "" {
			return FieldError(CodeInvalidInput, "ingredients.unit", "is required")
		}
		if len([]rune(ingredient.Unit)) > 16 {
			return FieldError(CodeInvalidInput, "ingredients.unit",
				"must not exceed 16 characters")
		}
		if math.IsNaN(ingredient.QuantityPerLiter) || math.IsInf(ingredient.QuantityPerLiter, 0) ||
			ingredient.QuantityPerLiter <= 0 {
			return FieldError(CodeInvalidInput, "ingredients.quantity_per_liter",
				"must be a finite number greater than zero")
		}
		if ingredient.QuantityPerLiter > 1_000_000 {
			return FieldError(CodeInvalidInput, "ingredients.quantity_per_liter",
				"must not exceed 1000000")
		}
	}
	return nil
}

func SnapshotRunIngredients(ingredients []FormulaIngredient, plannedVolumeL float64) []RunIngredient {
	snapshot := make([]RunIngredient, 0, len(ingredients))
	for _, ingredient := range ingredients {
		snapshot = append(snapshot, RunIngredient{
			Sequence:         ingredient.Sequence,
			Name:             ingredient.Name,
			Unit:             ingredient.Unit,
			QuantityPerLiter: ingredient.QuantityPerLiter,
			PlannedQuantity:  round3(ingredient.QuantityPerLiter * plannedVolumeL),
		})
	}
	return snapshot
}

func round3(value float64) float64 {
	return math.Round(value*1000) / 1000
}
