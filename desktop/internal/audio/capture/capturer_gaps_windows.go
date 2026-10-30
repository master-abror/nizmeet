//go:build windows

package capture

import "time"

// TakeGapIntervalMax mengembalikan celah antarpaket terbesar sejak pemanggilan
// sebelumnya, lalu meresetnya. Hanya untuk satu pencatat (alat ukur).
func (c *Capturer) TakeGapIntervalMax() time.Duration {
	c.mu.Lock()
	p := c.pipe
	c.mu.Unlock()
	if p == nil {
		return 0
	}
	return p.GapIntervalMax()
}
