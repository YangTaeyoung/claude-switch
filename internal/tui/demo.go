package tui

import (
	"context"
	"time"

	"github.com/YangTaeyoung/claude-switch/internal/limit"
)

// demoBackend는 GIF 녹화·시연용 가짜 백엔드다. 실제 키체인·계정 정보를 일절 쓰지 않는다.
type demoBackend struct {
	profiles []Profile
	active   string
}

func newDemoBackend() *demoBackend {
	return &demoBackend{
		profiles: []Profile{
			{Name: "work", Email: "work@example.com"},
			{Name: "personal", Email: "personal@example.com"},
		},
		active: "work",
	}
}

func (d *demoBackend) Profiles() ([]Profile, string, error) { return d.profiles, d.active, nil }

func (d *demoBackend) Switch(name string) error {
	d.active = name
	return nil
}

func (d *demoBackend) Delete(name string) error { return nil }

func (d *demoBackend) Usage(ctx context.Context, name string) limit.Result {
	time.Sleep(900 * time.Millisecond) // 스피너 시연
	now := time.Now()
	if name == "work" {
		return limit.Result{
			Status:   "allowed",
			FiveHour: limit.Window{Status: "allowed", Utilization: 0.82, ResetsAt: now.Add(90 * time.Minute)},
			SevenDay: limit.Window{Status: "allowed", Utilization: 0.12, ResetsAt: now.Add(6 * 24 * time.Hour)},
		}
	}
	return limit.Result{
		Status:   "allowed",
		FiveHour: limit.Window{Status: "allowed", Utilization: 0.03, ResetsAt: now.Add(3 * time.Hour)},
		SevenDay: limit.Window{Status: "allowed", Utilization: 0.41, ResetsAt: now.Add(2 * 24 * time.Hour)},
	}
}
