package model

type TravelDetails struct {
	Travel   Travel    `json:"travel"`
	Segments []Segment `json:"segments"`
}

func (td *TravelDetails) GetDestinationSegment() *Segment {
	// find the last outward segment
	maxOutwardSegment := -1
	destinationSegment := Segment{}
	for i := range td.Segments {
		if td.Segments[i].IsOutward && td.Segments[i].NumSegment > maxOutwardSegment {
			maxOutwardSegment = td.Segments[i].NumSegment
			destinationSegment = td.Segments[i]
		}
	}

	if maxOutwardSegment == -1 {
		return nil
	} else {
		return &destinationSegment
	}
}
