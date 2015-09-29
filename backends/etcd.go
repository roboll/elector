package backends

import (
	"log"
	"os"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/datawisesystems/etcd-lock"
)

import "github.com/roboll/elector/elector"

// Etcd is an election backend powered by etcd.
type Etcd struct {
	Members *[]string

	InstanceID      *string
	Keyspace        *string
	TTL             *time.Duration
	RefreshInterval *time.Duration

	CAFile   *string
	CertFile *string
	KeyFile  *string
}

// ElectionLoop uses etcd to elect a leader based on a configured key.
func (e *Etcd) ElectionLoop(updates chan elector.State) error {
	err := e.setFallbackOptions()
	if err != nil {
		return err
	}

	updates <- elector.StateNotLeader

	etcdClient, err := newEtcdClient(e)
	if err != nil {
		return err
	}

	lock, err := utils.NewMaster(etcdClient, *e.Keyspace, *e.InstanceID, 30)
	go e.waitForChanges(updates, lock.EventsChan())

	log.Println("etcd: starting lock acquisition")
	lock.Start()

	return nil
}

func (e *Etcd) waitForChanges(updates chan elector.State, eventsCh <-chan utils.MasterEvent) {
	for {
		select {
		case evt := <-eventsCh:
			if evt.Type == utils.MasterAdded {
				log.Println("etcd: rcvd a master added event")
				updates <- elector.StateLeader
			} else if evt.Type == utils.MasterDeleted {
				log.Println("etcd: rcvd a master deleted event.")
				updates <- elector.StateNotLeader
			} else {
				log.Println("etcd: leader changed. doing nothing.")
			}
		}
	}
}

func (e *Etcd) setFallbackOptions() error {
	if e.TTL == nil {
		log.Println("etcd: TTL not set, falling back to default (60s)")
		ttl := 1 * time.Minute
		e.TTL = &ttl
	}

	if e.RefreshInterval == nil {
		log.Println("etcd: RefreshInterval not set, falling back to default (TTL / 2)")
		interval := *e.TTL / 2
		e.RefreshInterval = &interval
	}

	if e.InstanceID == nil {
		log.Println("etcd: InstanceID not set, falling back to hostname")
		hostname, err := os.Hostname()
		if err != nil {
			return err
		}
		log.Printf("etcd: using %s as InstanceID", hostname)
		e.InstanceID = &hostname
	}

	return nil
}

func newEtcdClient(e *Etcd) (utils.Registry, error) {
	var c utils.Registry
	// if all are set, try to use tls
	if *e.CertFile != "" && *e.KeyFile != "" && *e.CAFile != "" {
		return etcd.NewTLSClient(*e.Members, *e.CertFile, *e.KeyFile, *e.CAFile)
	}
	c = etcd.NewClient(*e.Members)

	return c, nil
}
