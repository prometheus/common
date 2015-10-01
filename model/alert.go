package model

import (
	"time"
)

type Alert struct {
	// Label value pairs for purpose of aggregation, matching, and disposition
	// dispatching. This must minimally include an "alertname" label.
	Labels LabelSet `json:"labels"`

	// Extra key/value information which does not define alert identity.
	Annotations LabelSet `json:"annotations"`

	// The known time range for this alert. Both ends are optional.
	StartsAt time.Time `json:"startsAt,omitempty"`
	EndsAt   time.Time `json:"endsAt,omitempty"`
}
