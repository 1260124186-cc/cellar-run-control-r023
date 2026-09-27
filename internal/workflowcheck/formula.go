package workflowcheck

import (
	"fmt"

	"github.com/1260124186-cc/solo-0016-cellar-run-control/internal/domain"
)

func formulaApproval(api *client) error {
	badPayload := createFormulaPayload("Broken Recipe")
	badPayload["ingredients"] = []map[string]any{
		{"sequence": 1, "name": "Pale malt", "grams_per_liter": -5},
	}
	badCreate, err := api.call("POST", "/v1/formulas", badPayload, nil)
	if err != nil {
		return err
	}
	if err := requireCode(badCreate, 400, string(domain.CodeInvalidInput),
		"create formula with invalid ingredient"); err != nil {
		return err
	}

	createdResponse, err := api.call("POST", "/v1/formulas",
		createFormulaPayload("Current Pale"), nil)
	if err != nil {
		return err
	}
	if err := requireStatus(createdResponse, 201, "create formula"); err != nil {
		return err
	}
	var formula formulaView
	if err := createdResponse.decode(&formula); err != nil {
		return err
	}
	if formula.State != domain.FormulaDraft || formula.Version != 1 {
		return fmt.Errorf("created formula has unexpected state or version")
	}
	if len(formula.Ingredients) != 0 {
		return fmt.Errorf("new draft formula should start with no ingredients")
	}

	if err := rejectBadIngredients(api, formula.ID); err != nil {
		return err
	}

	premature, err := api.call("POST", "/v1/formulas/"+formula.ID+"/approve", nil, ptrInt64(1))
	if err != nil {
		return err
	}
	if err := requireCode(premature, 400, string(domain.CodeInvalidInput),
		"approve without steps or ingredients"); err != nil {
		return err
	}

	stepsResponse, err := api.call("PUT", "/v1/formulas/"+formula.ID+"/steps",
		formulaStepPayload(), ptrInt64(1))
	if err != nil {
		return err
	}
	if err := requireStatus(stepsResponse, 200, "set formula steps"); err != nil {
		return err
	}

	approveWithoutIngredients, err := api.call("POST",
		"/v1/formulas/"+formula.ID+"/approve", nil, ptrInt64(2))
	if err != nil {
		return err
	}
	if err := requireCode(approveWithoutIngredients, 400, string(domain.CodeInvalidInput),
		"approve without ingredients"); err != nil {
		return err
	}

	ingredientsResponse, err := api.call("PUT",
		"/v1/formulas/"+formula.ID+"/ingredients",
		formulaIngredientPayload(), ptrInt64(2))
	if err != nil {
		return err
	}
	if err := requireStatus(ingredientsResponse, 200, "set formula ingredients"); err != nil {
		return err
	}
	if err := ingredientsResponse.decode(&formula); err != nil {
		return err
	}
	if formula.Version != 3 || len(formula.Ingredients) != 3 {
		return fmt.Errorf("formula ingredients were not accepted as one revision")
	}

	duplicate, err := api.call("POST", "/v1/formulas",
		createFormulaPayload("  current   pale  "), nil)
	if err != nil {
		return err
	}
	if err := requireCode(duplicate, 409, string(domain.CodeFormulaNameConflict),
		"create duplicate formula"); err != nil {
		return err
	}

	approvedResponse, err := api.call("POST", "/v1/formulas/"+formula.ID+"/approve",
		nil, ptrInt64(3))
	if err != nil {
		return err
	}
	if err := requireStatus(approvedResponse, 200, "approve formula"); err != nil {
		return err
	}
	if err := approvedResponse.decode(&formula); err != nil {
		return err
	}
	if formula.State != domain.FormulaApproved || formula.Version != 4 {
		return fmt.Errorf("approved formula has unexpected state or version")
	}
	if len(formula.Ingredients) != 3 {
		return fmt.Errorf("approved formula lost its ingredient list")
	}

	if err := rejectApprovedIngredientChange(api, formula.ID, formula.Version); err != nil {
		return err
	}

	readResponse, err := api.call("GET", "/v1/formulas/"+formula.ID, nil, nil)
	if err != nil {
		return err
	}
	if err := requireStatus(readResponse, 200, "read formula"); err != nil {
		return err
	}
	var readBack formulaView
	if err := readResponse.decode(&readBack); err != nil {
		return err
	}
	if readBack.State != domain.FormulaApproved ||
		len(readBack.Steps) != 3 || len(readBack.Ingredients) != 3 {
		return fmt.Errorf("approved formula was not readable with its full recipe")
	}
	if readBack.Ingredients[1].Name != "Cascade hops" ||
		readBack.Ingredients[1].GramsPerLiter != 1.5 {
		return fmt.Errorf("approved formula did not retain ordered ingredients")
	}

	repeated, err := api.call("POST", "/v1/formulas/"+formula.ID+"/approve", nil, ptrInt64(4))
	if err != nil {
		return err
	}
	if err := requireCode(repeated, 409, string(domain.CodeFormulaState),
		"approve formula twice"); err != nil {
		return err
	}
	return nil
}

func rejectBadIngredients(api *client, formulaID string) error {
	zeroQuantity, err := api.call("PUT", "/v1/formulas/"+formulaID+"/ingredients",
		map[string]any{"ingredients": []map[string]any{
			{"sequence": 1, "name": "Water", "grams_per_liter": 0},
		}}, ptrInt64(1))
	if err != nil {
		return err
	}
	if err := requireCode(zeroQuantity, 400, string(domain.CodeInvalidInput),
		"reject non-positive ingredient quantity"); err != nil {
		return err
	}

	duplicateSequence, err := api.call("PUT",
		"/v1/formulas/"+formulaID+"/ingredients",
		map[string]any{"ingredients": []map[string]any{
			{"sequence": 1, "name": "Pale malt", "grams_per_liter": 200},
			{"sequence": 1, "name": "Cascade hops", "grams_per_liter": 1.5},
		}}, ptrInt64(1))
	if err != nil {
		return err
	}
	if err := requireCode(duplicateSequence, 400, string(domain.CodeInvalidInput),
		"reject duplicate ingredient sequence"); err != nil {
		return err
	}

	missingName, err := api.call("PUT", "/v1/formulas/"+formulaID+"/ingredients",
		map[string]any{"ingredients": []map[string]any{
			{"sequence": 1, "name": "   ", "grams_per_liter": 10},
		}}, ptrInt64(1))
	if err != nil {
		return err
	}
	if err := requireCode(missingName, 400, string(domain.CodeInvalidInput),
		"reject empty ingredient name"); err != nil {
		return err
	}

	unchanged, err := api.call("GET", "/v1/formulas/"+formulaID, nil, nil)
	if err != nil {
		return err
	}
	var formula formulaView
	if err := unchanged.decode(&formula); err != nil {
		return err
	}
	if formula.Version != 1 || len(formula.Ingredients) != 0 {
		return fmt.Errorf("rejected ingredient writes must not change the draft")
	}
	return nil
}

func rejectApprovedIngredientChange(api *client, formulaID string, version int64) error {
	response, err := api.call("PUT", "/v1/formulas/"+formulaID+"/ingredients",
		formulaIngredientPayload(), ptrInt64(version))
	if err != nil {
		return err
	}
	if err := requireCode(response, 409, string(domain.CodeFormulaState),
		"change approved ingredient list"); err != nil {
		return err
	}

	stale, err := api.call("GET", "/v1/formulas/"+formulaID, nil, nil)
	if err != nil {
		return err
	}
	var formula formulaView
	if err := stale.decode(&formula); err != nil {
		return err
	}
	if formula.Version != version || len(formula.Ingredients) != 3 {
		return fmt.Errorf("rejected approved-list write must leave the formula intact")
	}
	return nil
}

func ensureApprovedFormula(api *client, name string) (formulaView, error) {
	createdResponse, err := api.call("POST", "/v1/formulas", createFormulaPayload(name), nil)
	if err != nil {
		return formulaView{}, err
	}
	if err := requireStatus(createdResponse, 201, "create setup formula"); err != nil {
		return formulaView{}, err
	}
	var formula formulaView
	if err := createdResponse.decode(&formula); err != nil {
		return formulaView{}, err
	}
	stepsResponse, err := api.call("PUT", "/v1/formulas/"+formula.ID+"/steps",
		formulaStepPayload(), ptrInt64(1))
	if err != nil {
		return formulaView{}, err
	}
	if err := requireStatus(stepsResponse, 200, "set setup steps"); err != nil {
		return formulaView{}, err
	}
	ingredientsResponse, err := api.call("PUT",
		"/v1/formulas/"+formula.ID+"/ingredients",
		formulaIngredientPayload(), ptrInt64(2))
	if err != nil {
		return formulaView{}, err
	}
	if err := requireStatus(ingredientsResponse, 200, "set setup ingredients"); err != nil {
		return formulaView{}, err
	}
	approveResponse, err := api.call("POST", "/v1/formulas/"+formula.ID+"/approve",
		nil, ptrInt64(3))
	if err != nil {
		return formulaView{}, err
	}
	if err := requireStatus(approveResponse, 200, "approve setup formula"); err != nil {
		return formulaView{}, err
	}
	if err := approveResponse.decode(&formula); err != nil {
		return formulaView{}, err
	}
	return formula, nil
}

func ptrInt64(value int64) *int64 {
	return &value
}
