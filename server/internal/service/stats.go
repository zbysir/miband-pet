package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Stats 是每天完成了多少个任务（一个 agent 从 working 变成 done/idle 算一个），存在 stats.json。
// 每完成一个就得一条小鱼干，在手环上点一下喂给猫。
type Stats struct {
	path string

	mu       sync.Mutex
	Days     map[string]*DayStats `json:"days"`     // 日期（2006-01-02）→ 当天的数
	FishLeft int                  `json:"fishLeft"` // 还没喂的小鱼干
	FedTotal int                  `json:"fedTotal"` // 一共喂了多少条
}

type DayStats struct {
	Done int `json:"done"` // 完成了几个任务（得了几条小鱼干）
	Fed  int `json:"fed"`  // 喂了几条
}

// StatsView 是推给手环插件（useStore('stats')）和 App 界面的样子。
type StatsView struct {
	Today    DayStats  `json:"today"`
	FishLeft int       `json:"fishLeft"`
	FedTotal int       `json:"fedTotal"`
	Days     []DayView `json:"days"` // 最近 7 天，最后一个是今天
}

type DayView struct {
	Date    string `json:"date"`    // 10-22
	Weekday string `json:"weekday"` // 一 … 日
	Done    int    `json:"done"`
}

func LoadStats(path string) *Stats {
	s := &Stats{path: path, Days: map[string]*DayStats{}}
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, s)
		if s.Days == nil {
			s.Days = map[string]*DayStats{}
		}
	}
	return s
}

func dayKey(t time.Time) string { return t.Format("2006-01-02") }

func (s *Stats) day(t time.Time) *DayStats {
	k := dayKey(t)
	d := s.Days[k]
	if d == nil {
		d = &DayStats{}
		s.Days[k] = d
	}
	return d
}

// AddDone 记 n 个完成的任务（每个得一条小鱼干）。
func (s *Stats) AddDone(n int, now time.Time) {
	if n <= 0 {
		return
	}
	s.mu.Lock()
	s.day(now).Done += n
	s.FishLeft += n
	s.mu.Unlock()
	s.save()
}

var errNoFish = errors.New("没有小鱼干了")

// Feed 喂一条小鱼干。
func (s *Stats) Feed(now time.Time) error {
	s.mu.Lock()
	if s.FishLeft <= 0 {
		s.mu.Unlock()
		return errNoFish
	}
	s.FishLeft--
	s.FedTotal++
	s.day(now).Fed++
	s.mu.Unlock()
	s.save()
	return nil
}

func (s *Stats) View(now time.Time) StatsView {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := StatsView{FishLeft: s.FishLeft, FedTotal: s.FedTotal}
	if d := s.Days[dayKey(now)]; d != nil {
		v.Today = *d
	}
	weekdays := []string{"日", "一", "二", "三", "四", "五", "六"}
	for i := 6; i >= 0; i-- {
		t := now.AddDate(0, 0, -i)
		dv := DayView{Date: t.Format("01-02"), Weekday: weekdays[t.Weekday()]}
		if d := s.Days[dayKey(t)]; d != nil {
			dv.Done = d.Done
		}
		v.Days = append(v.Days, dv)
	}
	return v
}

func (s *Stats) save() {
	s.mu.Lock()
	raw, err := json.MarshalIndent(s, "", "  ")
	s.mu.Unlock()
	if err != nil || s.path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.path), 0o700)
	tmp := s.path + ".tmp"
	if os.WriteFile(tmp, raw, 0o600) == nil {
		_ = os.Rename(tmp, s.path)
	}
}
