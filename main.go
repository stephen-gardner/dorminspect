package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/nlopes/slack"
	"github.com/pborman/getopt/v2"
)

var (
	db             *gorm.DB
	fullyRandom    = true
	numInspections = 5
	dormEndpoint   string
	slackToken     string
	channelID      string
)

func initDatabase() {
	var err error = nil
	db, err = gorm.Open("sqlite3", "nostromo.db")
	if err != nil {
		log.Fatal(err)
	}
	db.AutoMigrate(&Room{})
	db.AutoMigrate(&Dorm{})
}

func initEnvironment() {
	if envVar, present := os.LookupEnv("DORMINSPECT_NUM_INSPECTIONS"); present {
		if val, err := strconv.ParseInt(envVar, 10, 32); err == nil {
			numInspections = int(val)
		}
	}
	if envVar, present := os.LookupEnv("DORMINSPECT_FULLY_RANDOM"); present {
		if val, err := strconv.ParseBool(envVar); err == nil {
			fullyRandom = val
		}
	}
	dormEndpoint = os.Getenv("DORMINSPECT_DORM_ENDPOINT")
	slackToken = os.Getenv("DORMINSPECT_SLACK_TOKEN")
	channelID = os.Getenv("DORMINSPECT_CHANNEL_ID")
}

func breakdownData(lines []string) map[string]map[string]bool {
	occupants := make(map[string]map[string]bool)
	for _, entry := range lines {
		// Detects broken entries in Nostromo :(
		if len(entry) > 0 && entry[len(entry)-1] != ';' {
			log.Printf("Problem room: %s\n", entry)
		}

		pair := strings.Split(entry, ";")
		if len(pair) > 2 {
			tenant := pair[0]
			roomNumber := pair[1]
			if _, present := occupants[roomNumber]; !present {
				occupants[roomNumber] = make(map[string]bool)
			}
			occupants[roomNumber][tenant] = true
		}
	}
	return occupants
}

func getOccupants() map[string]map[string]bool {
	resp, err := http.Get(dormEndpoint)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	return breakdownData(strings.Split(string(body), "<br>"))
}

func execute(dorm *Dorm) {
	addRoom := getopt.String('a', "", "Add room to inspection list")
	removeRoom := getopt.String('r', "", "Remove room from inspection list")
	consoleFlag := getopt.Bool('c', "Send output to console")
	getopt.Lookup('c').SetOptional()
	duplicatesFlag := getopt.Bool('d', "Output list of tenants located in multiple rooms")
	getopt.Lookup('d').SetOptional()
	inspectFlag := getopt.Bool('i', "Output inspection list")
	getopt.Lookup('i').SetOptional()
	scheduleFlag := getopt.Bool('s', "Schedule inspection")
	getopt.Lookup('s').SetOptional()
	tenantsFlag := getopt.Bool('t', "Output list of rooms and tenants")
	getopt.Lookup('t').SetOptional()
	getopt.Parse()

	var out string
	hasOutput := false

	if len(*addRoom) > 0 {
		dorm.addToInspection(*addRoom)
	}

	if len(*removeRoom) > 0 {
		dorm.removeFromInspection(*removeRoom)
	}

	if *tenantsFlag {
		out += dorm.getTenants(!*consoleFlag)
		hasOutput = true
	}

	if *duplicatesFlag {
		if hasOutput {
			out += "\n"
		}
		out += dorm.getDuplicates(!*consoleFlag)
		hasOutput = true
	}

	if *inspectFlag || *scheduleFlag {
		if hasOutput {
			out += "\n"
		}
		out += dorm.getInspectionList(*scheduleFlag, !*consoleFlag)
		hasOutput = true
	}

	if !hasOutput {
		return
	}

	if *consoleFlag {
		fmt.Printf(out)
	} else {
		(&outgoing{
			api:       slack.New(slackToken),
			channelID: channelID,
			msg:       out,
		}).send()
	}
}

func main() {
	initEnvironment()
	initDatabase()
	defer db.Close()
	dorm := Dorm{}
	dorm.initRooms(db, getOccupants())
	execute(&dorm)
}
