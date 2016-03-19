package common

import "time"

// TimeWheel expiry handler...
type TimeWheel struct {
	ttl          time.Duration
	tick         time.Duration
	evictHandler timeWheelEvictHandler
	wheel        [][]*timeWheelEntry
	index        map[string]*timeWheelEntry
	prev         time.Time
}

type timeWheelEntry struct {
	key     string
	updated time.Time
}

type timeWheelEvictHandler func(string)

// NewTimeWheel ...
func NewTimeWheel(ttl, tick time.Duration, evictHandler timeWheelEvictHandler) *TimeWheel {
	return &TimeWheel{
		ttl,
		tick,
		evictHandler,
		make([][]*timeWheelEntry, ttl/tick),
		make(map[string]*timeWheelEntry, 0),
		time.Now(),
	}
}

// Update mark a key as updated
func (w *TimeWheel) Update(key string) {
	now := time.Now()
	updated := []*timeWheelEntry{}
	e, ok := w.index[key]
	if !ok {
		e = &timeWheelEntry{key, now}
		w.index[key] = e
		updated = append(updated, e)
	}
	e.updated = now
	d := int(now.Sub(w.prev) / w.tick)
	w.prev = w.prev.Add(time.Duration(d) * w.tick)
	if d > cap(w.wheel) {
		d = cap(w.wheel)
	}
	for i := 1; i <= d; i++ {
		l := w.wheel[cap(w.wheel)-i]
		if l == nil {
			continue
		}
		for j := 0; j < len(l); j++ {
			e := l[j]
			if now.Sub(e.updated) >= w.ttl {
				delete(w.index, e.key)
				w.evictHandler(e.key)
			} else {
				updated = append(updated, e)
			}
		}
	}
	copy(w.wheel[d:], w.wheel)
	for i := 0; i < d; i++ {
		w.wheel[i] = nil
	}
	for i := 0; i < len(updated); i++ {
		e := updated[i]
		ed := now.Sub(e.updated) / w.ttl
		if w.wheel[ed] == nil {
			w.wheel[ed] = []*timeWheelEntry{e}
		} else {
			w.wheel[ed] = append(w.wheel[ed], e)
		}
	}
}
