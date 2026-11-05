package domain

// BuildingRule describes address structure rather than individual residences.
// It lets the editor generate valid unit/floor/room choices without retaining
// a resident-level address directory.
type BuildingRule struct {
	ID                 string `json:"id"`
	Community          string `json:"community"`
	BuildingNo         string `json:"buildingNo"`
	UnitCount          int    `json:"unitCount"`
	FloorStart         int    `json:"floorStart"`
	FloorCount         int    `json:"floorCount"`
	HouseholdsPerFloor int    `json:"householdsPerFloor"`
}
