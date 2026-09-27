package workflowcheck

import (
	"fmt"
	"math"
	"time"

	"github.com/1260124186-cc/solo-0016-cellar-run-control/internal/domain"
)

type formulaView struct {
	ID          string                     `json:"id"`
	State       domain.FormulaState        `json:"state"`
	Version     int64                      `json:"version"`
	Steps       []domain.FormulaStep       `json:"steps"`
	Ingredients []domain.FormulaIngredient `json:"ingredients"`
}

type vesselView struct {
	ID          string             `json:"id"`
	State       domain.VesselState `json:"state"`
	Version     int64              `json:"version"`
	ActiveRunID *string            `json:"active_run_id"`
}

type runView struct {
	ID             string                      `json:"id"`
	State          domain.RunState             `json:"state"`
	Version        int64                       `json:"version"`
	VesselID       *string                     `json:"vessel_id"`
	FormulaVersion int64                       `json:"formula_version"`
	PlannedVolumeL float64                     `json:"planned_volume_liters"`
	Ingredients    []domain.RunIngredient      `json:"ingredients"`
	Observations   []domain.GravityObservation `json:"observations"`
	Metrics        *domain.RunMetrics          `json:"metrics"`
}

type pairView struct {
	Run    runView    `json:"run"`
	Vessel vesselView `json:"vessel"`
}

func createFormulaPayload(name string) map[string]any {
	return map[string]any{
		"name":                    name,
		"style":                   "Cellar pilot",
		"target_original_gravity": 1.052,
		"target_final_gravity":    1.012,
		"max_fermentation_temp_c": 22.0,
		"minimum_days":            7,
		"maximum_days":            21,
	}
}

func formulaStepPayload() map[string]any {
	return map[string]any{
		"steps": []map[string]any{
			{
				"sequence":      1,
				"label":         "Heat and combine",
				"temperature_c": 66.0,
				"hold_minutes":  60,
			},
			{
				"sequence":      2,
				"label":         "Transfer and cool",
				"temperature_c": 20.0,
				"hold_minutes":  30,
			},
			{
				"sequence":      3,
				"label":         "Hold for primary activity",
				"temperature_c": 19.0,
				"hold_minutes":  240,
			},
		},
	}
}

func formulaIngredientPayload() map[string]any {
	return map[string]any{
		"ingredients": []map[string]any{
			{
				"sequence":           1,
				"name":               "Pale malt",
				"unit":               "kg",
				"quantity_per_liter": 0.21,
			},
			{
				"sequence":           2,
				"name":               "Cascade hops",
				"unit":               "g",
				"quantity_per_liter": 1.8,
			},
		},
	}
}

func createRunPayload(code, formulaID string) map[string]any {
	return map[string]any{
		"code":                  code,
		"formula_id":            formulaID,
		"planned_volume_liters": 180.0,
		"notes":                 "pilot run",
	}
}

func verifiedAt(base time.Time, sequence int) time.Time {
	return base.Add(time.Duration(sequence) * 10 * time.Second)
}

func requireRunState(run runView, state domain.RunState) error {
	if run.State != state {
		return fmt.Errorf("expected run state %q, got %q", state, run.State)
	}
	return nil
}

func requireVesselState(vessel vesselView, state domain.VesselState) error {
	if vessel.State != state {
		return fmt.Errorf("expected vessel state %q, got %q", state, vessel.State)
	}
	return nil
}

func requireIngredientSnapshot(run runView, formula formulaView) error {
	if len(formula.Ingredients) == 0 {
		return errInvalidWorkflow("approved formula has no ingredient list")
	}
	if len(run.Ingredients) != len(formula.Ingredients) {
		return errInvalidWorkflow("run ingredient snapshot does not match the formula list")
	}
	for index, item := range run.Ingredients {
		source := formula.Ingredients[index]
		if item.Sequence != source.Sequence || item.Name != source.Name ||
			item.Unit != source.Unit {
			return errInvalidWorkflow("run ingredient does not identify its formula source")
		}
		if math.Abs(item.QuantityPerLiter-source.QuantityPerLiter) > 1e-9 {
			return errInvalidWorkflow("run ingredient rate does not match the formula")
		}
		expected := math.Round(source.QuantityPerLiter*run.PlannedVolumeL*1000) / 1000
		if math.Abs(item.PlannedQuantity-expected) > 1e-9 {
			return errInvalidWorkflow("run ingredient quantity was not computed from planned volume")
		}
	}
	return nil
}
