package apollo

import (
	"github.com/micro/go-micro/v2/config/encoder"
	"github.com/micro/go-micro/v2/config/source"
	"github.com/zouyx/agollo/v4/storage"
	"time"
)

type watcher struct {
	e    encoder.Encoder
	name string
	ch   chan *source.ChangeSet
	exit chan bool
}

func (w *watcher) OnNewestChange(event *storage.FullChangeEvent) {
	kv := map[string]string{}
	for k, v := range event.Changes {
		kv[k] = v.(string)
	}

	d, err := makeMap(w.e, kv)
	if err != nil {
		return
	}

	b, err := w.e.Encode(d)
	if err != nil {
		return
	}

	cs := &source.ChangeSet{
		Timestamp: time.Now(),
		Format:    w.e.String(),
		Source:    w.name,
		Data:      b,
	}
	cs.Checksum = cs.Sum()
	w.ch <- cs
}

func (w *watcher) OnChange(changeEvent *storage.ChangeEvent) {

}

func (w *watcher) Next() (*source.ChangeSet, error) {
	select {
	case cs := <-w.ch:
		return cs, nil
	case <-w.exit:
		return nil, source.ErrWatcherStopped
	}
}

func (w *watcher) Stop() error {
	select {
	case <-w.exit:
		return nil
	default:
	}
	return nil
}

func newWatcher(name string, e encoder.Encoder) (*watcher, error) {
	return &watcher{
		e:    e,
		name: name,
		exit: make(chan bool),
		ch:   make(chan *source.ChangeSet),
	}, nil
}
