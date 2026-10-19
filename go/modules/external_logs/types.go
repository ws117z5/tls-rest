package external_logs

import "time"

// ExternalLogObj is the stored shape of a log row.
type ExternalLogObj struct {
	ID         int64     `json:"id" db:"id"`
	UUID       string    `json:"uuid" db:"uuid"`
	App        string    `json:"app" db:"app"`
	Level      string    `json:"level" db:"level"`
	Message    string    `json:"message" db:"message"`
	StackTrace string    `json:"stack_trace" db:"stack_trace"`
	AppVersion string    `json:"app_version" db:"app_version"`
	DeviceInfo string    `json:"device_info" db:"device_info"`
	Created    time.Time `json:"created" db:"created"`
	Updated    time.Time `json:"updated" db:"updated"`
	CreatedBy  int       `json:"created_by" db:"created_by"`
}

func (ExternalLogObj) TableName() string { return "external_logs" }
