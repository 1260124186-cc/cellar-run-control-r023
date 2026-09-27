package workflowcheck

import (
	"fmt"
	"math"

	"github.com/1260124186-cc/solo-0016-cellar-run-control/internal/domain"
)

func vesselRunLifecycle(api *client) error {
	formula, err := ensureApprovedFormula(api, "Lifecycle Formula")
	if err != nil {
		return err
	}
	vesselResponse, err := api.call("POST", "/v1/vessels", map[string]any{
		"code":            "VSL-101",
		"capacity_liters": 240.0,
	}, nil)
	if err != nil {
		return err
	}
	if err := requireStatus(vesselResponse, 201, "create vessel"); err != nil {
		return err
	}
	var vessel vesselView
	if err := vesselResponse.decode(&vessel); err != nil {
		return err
	}

	firstResponse, err := api.call("POST", "/v1/runs",
		createRunPayload("RUN-LIFECYCLE-A", formula.ID), nil)
	if err != nil {
		return err
	}
	if err := requireStatus(firstResponse, 201, "create first run"); err != nil {
		return err
	}
	var first runView
	if err := firstResponse.decode(&first); err != nil {
		return err
	}
	if err := requireRunRecipe(first, formula); err != nil {
		return err
	}

	secondResponse, err := api.call("POST", "/v1/runs",
		createRunPayload("RUN-LIFECYCLE-B", formula.ID), nil)
	if err != nil {
		return err
	}
	if err := requireStatus(secondResponse, 201, "create second run"); err != nil {
		return err
	}
	var second runView
	if err := secondResponse.decode(&second); err != nil {
		return err
	}

	reservedResponse, err := api.call("POST", "/v1/runs/"+first.ID+"/reserve",
		map[string]any{"vessel_id": vessel.ID}, ptrInt64(1))
	if err != nil {
		return err
	}
	if err := requireStatus(reservedResponse, 200, "reserve vessel"); err != nil {
		return err
	}
	var pair pairView
	if err := reservedResponse.decode(&pair); err != nil {
		return err
	}
	if err := requireRunState(pair.Run, domain.RunReserved); err != nil {
		return err
	}
	if err := requireVesselState(pair.Vessel, domain.VesselReserved); err != nil {
		return err
	}

	conflictResponse, err := api.call("POST", "/v1/runs/"+second.ID+"/reserve",
		map[string]any{"vessel_id": vessel.ID}, ptrInt64(1))
	if err != nil {
		return err
	}
	if err := requireCode(conflictResponse, 409, string(domain.CodeVesselUnavailable),
		"reserve occupied vessel"); err != nil {
		return err
	}

	startResponse, err := api.call("POST", "/v1/runs/"+first.ID+"/start",
		nil, ptrInt64(2))
	if err != nil {
		return err
	}
	if err := requireStatus(startResponse, 200, "start run"); err != nil {
		return err
	}
	if err := startResponse.decode(&pair); err != nil {
		return err
	}
	if err := requireRunState(pair.Run, domain.RunFermenting); err != nil {
		return err
	}
	if err := requireVesselState(pair.Vessel, domain.VesselInUse); err != nil {
		return err
	}
	if pair.Run.VesselID == nil || *pair.Run.VesselID != vessel.ID {
		return errInvalidWorkflow("run did not retain its reserved vessel")
	}

	readResponse, err := api.call("GET", "/v1/runs/"+first.ID, nil, nil)
	if err != nil {
		return err
	}
	if err := requireStatus(readResponse, 200, "read started run"); err != nil {
		return err
	}
	if err := readResponse.decode(&first); err != nil {
		return err
	}
	if err := requireRunState(first, domain.RunFermenting); err != nil {
		return err
	}
	if err := requireRunRecipe(first, formula); err != nil {
		return err
	}
	return nil
}

func requireRunRecipe(run runView, formula formulaView) error {
	if run.Recipe == nil {
		return errInvalidWorkflow("run did not retain a recipe snapshot")
	}
	snapshot := run.Recipe
	if snapshot.FormulaID != formula.ID {
		return errInvalidWorkflow("recipe snapshot references the wrong formula")
	}
	if snapshot.FormulaName != formula.Name {
		return errInvalidWorkflow("recipe snapshot did not capture the formula name")
	}
	if snapshot.FormulaVersion != formula.Version {
		return errInvalidWorkflow("recipe snapshot did not capture the approved version")
	}
	const plannedVolume = 180.0
	if snapshot.PlannedVolumeL != plannedVolume {
		return errInvalidWorkflow("recipe snapshot did not capture the planned volume")
	}
	if len(snapshot.Ingredients) != len(formula.Ingredients) {
		return errInvalidWorkflow("recipe snapshot ingredient count does not match the formula")
	}
	for index, usage := range snapshot.Ingredients {
		ingredient := formula.Ingredients[index]
		if usage.Sequence != ingredient.Sequence ||
			usage.IngredientName != ingredient.Name ||
			usage.GramsPerLiter != ingredient.GramsPerLiter {
			return errInvalidWorkflow("recipe line does not match its approved ingredient")
		}
		if usage.PlannedVolumeLiters != plannedVolume {
			return errInvalidWorkflow("recipe line did not retain its calculation volume")
		}
		expected := ingredient.GramsPerLiter * plannedVolume
		if math.Abs(usage.QuantityGrams-expected) > 0.0005 {
			return fmt.Errorf("recipe quantity for %q is %g, expected %g",
				usage.IngredientName, usage.QuantityGrams, expected)
		}
	}
	return nil
}

type workflowError string

func (e workflowError) Error() string {
	return string(e)
}

func errInvalidWorkflow(message string) error {
	return workflowError(message)
}
