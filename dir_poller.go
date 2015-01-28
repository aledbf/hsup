package hsup

import (
	"log"
	"time"
)

type DirPoller struct {
	Dir string
	Hs  *Startup

	c *conf

	lastReleaseID string
}

func newControlDir() interface{} {
	return &AppSerializable{}
}

func (dp *DirPoller) Notify() <-chan *Processes {
	out := make(chan *Processes)
	dp.c = newConf(newControlDir, dp.Dir)
	go dp.pollSynchronous(out)
	return out
}

func (dp *DirPoller) pollSynchronous(out chan<- *Processes) {
	for {
		var hs Startup

		newInfo, err := dp.c.Notify()
		if err != nil {
			log.Println("Could not fetch new release information:",
				err)
			goto wait
		}

		if !newInfo {
			goto wait
		}

		hs = Startup{
			App:     *dp.c.Snapshot().(*AppSerializable),
			Driver:  dp.Hs.Driver,
			OneShot: dp.Hs.OneShot,
		}
		out <- hs.Procs()
	wait:
		time.Sleep(10 * time.Second)
	}
}
