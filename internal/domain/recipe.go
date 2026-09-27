package domain

import (
	"math"
	"time"
)

// IngredientUsage is one immutable quantity line captured when a run is
// created. It records the approved ingredient it came from and the exact
// calculation basis so a run detail can be cross-explained against the
// formula detail at any later time.
type IngredientUsage struct {
	Sequence            int     `json:"sequence"`
	IngredientName      string  `json:"ingredient_name"`
	GramsPerLiter       float64 `json:"grams_per_liter"`
	PlannedVolumeLiters float64 `json:"planned_volume_liters"`
	QuantityGrams       float64 `json:"quantity_grams"`
}

// RecipeSnapshot is the frozen copy of the approved formula ingredient list
// and the computed per-batch quantities a run was created with.
type RecipeSnapshot struct {
	FormulaID      string            `json:"formula_id"`
	FormulaName    string            `json:"formula_name"`
	FormulaVersion int64             `json:"formula_version"`
	PlannedVolumeL float64           `json:"planned_volume_liters"`
	Ingredients    []IngredientUsage `json:"ingredients"`
	CapturedAt     time.Time         `json:"captured_at"`
}

func (s RecipeSnapshot) Clone() RecipeSnapshot {
	cloned := s
	cloned.Ingredients = append([]IngredientUsage(nil), s.Ingredients...)
	return cloned
}

// BuildRecipeSnapshot freezes the approved formula ingredient list and
// computes each quantity as grams_per_liter * planned_volume_liters, rounded
// to three decimal places. The formula must be approved and carry a complete
// valid ingredient list; the planned volume must be a positive finite value.
func BuildRecipeSnapshot(
	formula Formula,
	plannedVolumeL float64,
	now time.Time,
) (RecipeSnapshot, error) {
	if formula.State != FormulaApproved {
		return RecipeSnapshot{}, NewError(CodeFormulaState,
			"recipe snapshots require an approved formula")
	}
	if err := ValidateFormulaIngredients(formula.Ingredients, true); err != nil {
		return RecipeSnapshot{}, err
	}
	if math.IsNaN(plannedVolumeL) || math.IsInf(plannedVolumeL, 0) || plannedVolumeL <= 0 {
		return RecipeSnapshot{}, FieldError(CodeInvalidInput, "planned_volume_liters",
			"must be a positive finite value")
	}
	usages := make([]IngredientUsage, 0, len(formula.Ingredients))
	for _, ingredient := range formula.Ingredients {
		quantity := ingredient.GramsPerLiter * plannedVolumeL
		if math.IsInf(quantity, 0) || math.IsNaN(quantity) {
			return RecipeSnapshot{}, FieldError(CodeInvalidInput,
				"planned_volume_liters", "produces a quantity that cannot be represented")
		}
		usages = append(usages, IngredientUsage{
			Sequence:            ingredient.Sequence,
			IngredientName:      ingredient.Name,
			GramsPerLiter:       ingredient.GramsPerLiter,
			PlannedVolumeLiters: plannedVolumeL,
			QuantityGrams:       round3(quantity),
		})
	}
	return RecipeSnapshot{
		FormulaID:      formula.ID,
		FormulaName:    formula.Name,
		FormulaVersion: formula.Version,
		PlannedVolumeL: plannedVolumeL,
		Ingredients:    usages,
		CapturedAt:     now.UTC(),
	}, nil
}
