/*
Copyright 2018 The Kubernetes authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clients

import (
	"log"
	"time"

	"github.com/softlayer/softlayer-go/datatypes"
	"github.com/softlayer/softlayer-go/services"
	"github.com/softlayer/softlayer-go/session"
	"github.com/softlayer/softlayer-go/sl"
)

type GuestService struct {
	sess *session.Session
}

func NewGuestService(sess *session.Session) GuestService {
	return GuestService{sess: sess}
}

func (gs *GuestService) guestWaitReady(Id int) {
	// Wait for transactions to finish
	log.Printf("Waiting for transactions to complete before destroying.")
	s := services.GetVirtualGuestService(gs.sess).Id(Id)

	// Delay to allow transactions to be registered
	time.Sleep(5 * time.Second)

	for transactions, _ := s.GetActiveTransactions(); len(transactions) > 0; {
		log.Print(".")
		// TODO(gyliu513) make it configurable or use the notification mechanism to optimize
		// the process instead of hardcoded waiting.
		time.Sleep(5 * time.Second)
		transactions, _ = s.GetActiveTransactions()
	}
	log.Println("wait done")
}

func (gs *GuestService) GuestCreate(clusterName string, name string) {
	s := services.GetVirtualGuestService(gs.sess)

	// Create a Virtual_Guest instance as a template
	vGuestTemplate := datatypes.Virtual_Guest{
		Hostname:                     sl.String(name),
		Domain:                       sl.String("example.com"),
		MaxMemory:                    sl.Int(4096),
		StartCpus:                    sl.Int(1),
		Datacenter:                   &datatypes.Location{Name: sl.String("wdc01")},
		OperatingSystemReferenceCode: sl.String("UBUNTU_LATEST"),
		LocalDiskFlag:                sl.Bool(true),
		HourlyBillingFlag:            sl.Bool(true),
	}

	vGuest, err := s.Mask("id;domain").CreateObject(&vGuestTemplate)
	if err != nil {
		log.Printf("%s\n", err)
		return
	} else {
		log.Printf("\nNew Virtual Guest created with ID %d\n", *vGuest.Id)
		log.Printf("Domain: %s\n", *vGuest.Domain)
	}

	// Wait for transactions to finish
	log.Printf("Waiting for transactions to complete before destroying.")
	gs.guestWaitReady(*vGuest.Id)
}

func (gs *GuestService) GuestDelete(Id int) {
	s := services.GetVirtualGuestService(gs.sess).Id(Id)

	success, err := s.DeleteObject()
	if err != nil {
		log.Printf("Error deleting virtual guest: %s", err)
	} else if success == false {
		log.Printf("Error deleting virtual guest")
	} else {
		log.Printf("Virtual Guest deleted successfully")
	}
}

func (gs *GuestService) GuestList() ([]datatypes.Virtual_Guest, error) {
	s := services.GetAccountService(gs.sess)

	guests, err := s.GetVirtualGuests()
	if err != nil {
		log.Printf("Error listinging virtual guest: %s", err)
		return []datatypes.Virtual_Guest{}, err
	}
	return guests, nil
}
