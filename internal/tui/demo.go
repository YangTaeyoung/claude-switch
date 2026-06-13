package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/YangTaeyoung/claude-switch/internal/i18n"
	"github.com/YangTaeyoung/claude-switch/internal/limit"
)

// demoBackend는 GIF 녹화·시연용 가짜 백엔드다. 실제 키체인·계정 정보를 일절 쓰지 않는다.
type demoBackend struct {
	profiles   []Profile
	active     string
	lang       i18n.Lang
	autoUpdate bool
}

func newDemoBackend() *demoBackend {
	return &demoBackend{
		profiles: []Profile{
			{Name: "work", Email: "work@example.com"},
			{Name: "personal", Email: "personal@example.com"},
		},
		active: "work",
		lang:   i18n.EN,
	}
}

func (d *demoBackend) Profiles() ([]Profile, string, error) { return d.profiles, d.active, nil }

func (d *demoBackend) Save(name string) error {
	for _, p := range d.profiles {
		if p.Name == name {
			return fmt.Errorf("profile %q already exists", name)
		}
	}
	d.profiles = append(d.profiles, Profile{Name: name, Email: name + "@example.com"})
	d.active = name
	return nil
}

func (d *demoBackend) Switch(name string) error {
	d.active = name
	return nil
}

func (d *demoBackend) Rename(oldName, newName string) error {
	for i := range d.profiles {
		if d.profiles[i].Name == oldName {
			d.profiles[i].Name = newName
			if d.active == oldName {
				d.active = newName
			}
			return nil
		}
	}
	return fmt.Errorf("profile %q not found", oldName)
}

func (d *demoBackend) Delete(name string) error {
	for i := range d.profiles {
		if d.profiles[i].Name == name {
			d.profiles = append(d.profiles[:i], d.profiles[i+1:]...)
			return nil
		}
	}
	return nil
}

func (d *demoBackend) Language() i18n.Lang { return d.lang }

func (d *demoBackend) SetLanguage(l i18n.Lang) error {
	d.lang = l
	return nil
}

func (d *demoBackend) AutoUpdate() bool { return d.autoUpdate }

func (d *demoBackend) SetAutoUpdate(v bool) error {
	d.autoUpdate = v
	return nil
}

func (d *demoBackend) Usage(ctx context.Context, name string) limit.Result {
	time.Sleep(900 * time.Millisecond) // 스피너 시연
	now := time.Now()
	if name == d.profiles[0].Name {
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
