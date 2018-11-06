package main

import (
	"encoding/json"
	"fmt"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
	"github.corp.ebay.com/jwijnands/vulcan/config"
	"github.corp.ebay.com/jwijnands/vulcan/facter"
)

func main() {
	log.SetLevel(log.DebugLevel)

	facts, err := facter.New()
	if err != nil {
		log.Fatalf("error parsing facts: %s", err)
	}

	b, err := json.Marshal(facts)
	if err != nil {
		log.Fatalf("error marshalling facts: %s", err)
	}

	fmt.Printf("%s\n", b)

	cfg, err := config.LoadDir("/app/test")
	if err != nil {
		log.Fatalf("%s", err)
	}

	fmt.Println(spew.Sdump(cfg))
}
