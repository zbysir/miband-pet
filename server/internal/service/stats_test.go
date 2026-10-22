package service

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.json")
	day1 := time.Date(2026, 10, 21, 23, 0, 0, 0, time.Local)
	day2 := day1.Add(2 * time.Hour)
	s := LoadStats(path)
	s.AddDone(3, day1)
	s.AddDone(2, day2)
	if err := s.Feed(day2); err != nil {
		t.Fatal(err)
	}

	// 重新读文件，数还在
	s = LoadStats(path)
	v := s.View(day2)
	if v.Today != (DayStats{Done: 2, Fed: 1}) || v.FishLeft != 4 || v.FedTotal != 1 {
		t.Fatalf("不对：%+v", v)
	}
	if len(v.Days) != 7 || v.Days[6].Date != "10-22" || v.Days[6].Weekday != "四" || v.Days[5].Done != 3 {
		t.Fatalf("最近 7 天不对：%+v", v.Days)
	}
	for range 4 {
		if err := s.Feed(day2); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Feed(day2); err != errNoFish {
		t.Fatalf("没有小鱼干时应该喂不了：%v", err)
	}
}
