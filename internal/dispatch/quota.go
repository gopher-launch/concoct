package dispatch

import (
	"encoding/json"
	"fmt"
	"time"
)

// ParseQuota reads the verified 0.147.0 rate-limit response without deriving
// account spend from token usage or from changes between observations.
func ParseQuota(raw json.RawMessage, observedAt time.Time) (QuotaObservation, error) {
	var response struct {
		RateLimits          json.RawMessage            `json:"rateLimits"`
		RateLimitsByLimitID map[string]json.RawMessage `json:"rateLimitsByLimitId"`
	}
	if observedAt.IsZero() || json.Unmarshal(raw, &response) != nil || len(response.RateLimits) == 0 {
		return QuotaObservation{}, fmt.Errorf("malformed provider quota response")
	}
	observation := QuotaObservation{ObservedAt: observedAt, Windows: map[string]Window{}}
	if len(response.RateLimitsByLimitID) > 0 {
		for id, snapshot := range response.RateLimitsByLimitID {
			if err := parseBucket(observation.Windows, id+"/", snapshot); err != nil {
				return QuotaObservation{}, err
			}
		}
	} else if err := parseBucket(observation.Windows, "", response.RateLimits); err != nil {
		return QuotaObservation{}, err
	}
	return observation, nil
}

func parseBucket(windows map[string]Window, prefix string, raw json.RawMessage) error {
	var bucket struct {
		Primary   json.RawMessage `json:"primary"`
		Secondary json.RawMessage `json:"secondary"`
	}
	if json.Unmarshal(raw, &bucket) != nil {
		return fmt.Errorf("malformed provider quota bucket %q", prefix)
	}
	for name, value := range map[string]json.RawMessage{"primary": bucket.Primary, "secondary": bucket.Secondary} {
		if len(value) == 0 || string(value) == "null" {
			continue
		}
		var row struct {
			UsedPercent *float64 `json:"usedPercent"`
			ResetsAt    *int64   `json:"resetsAt"`
		}
		if json.Unmarshal(value, &row) != nil || row.UsedPercent == nil || *row.UsedPercent < 0 || *row.UsedPercent > 100 {
			return fmt.Errorf("malformed provider quota window %q", prefix+name)
		}
		window := Window{UsedPercent: *row.UsedPercent}
		if row.ResetsAt != nil {
			window.ResetAt = time.Unix(*row.ResetsAt, 0).UTC()
		}
		windows[prefix+name] = window
	}
	return nil
}
