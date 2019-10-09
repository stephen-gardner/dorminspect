package main

import (
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
)

type Dorm struct {
	gorm.Model
	ToInspect      string `gorm:"NOT NULL"`
	LastInspection int64  `gorm:"DEFAULT:0"`
	rooms          []Room `gorm:"-"`
}

func (dorm *Dorm) getRoom(roomNumber string) *Room {
	for _, room := range dorm.rooms {
		if room.RoomNumber == roomNumber {
			return &room
		}
	}
	return nil
}

func (dorm *Dorm) getRoomsForTenant(tenant string) []string {
	rooms := make([]string, 0)
	for _, room := range dorm.rooms {
		for _, person := range room.tenants {
			if person == tenant {
				rooms = append(rooms, room.RoomNumber)
			}
		}
	}
	sort.Strings(rooms)
	return rooms
}

func (dorm *Dorm) initRooms(db *gorm.DB, occupants map[string]map[string]bool) {
	for roomNumber := range occupants {
		room := Room{}

		db.Model(&room).Where("room_number = ?", roomNumber).First(&room)
		room.RoomNumber = roomNumber
		room.tenants = make([]string, 0)
		for tenant := range occupants[roomNumber] {
			room.tenants = append(room.tenants, tenant)
		}
		sort.Strings(room.tenants)
		db.Model(&room).Where("room_number = ?", roomNumber).Save(&room)
		dorm.rooms = append(dorm.rooms, room)
	}

	sort.Slice(dorm.rooms, func(i, j int) bool {
		return strings.Compare(dorm.rooms[i].RoomNumber, dorm.rooms[j].RoomNumber) < 0
	})
	db.Model(dorm).Where("id = 1").FirstOrCreate(dorm)
}

func (dorm *Dorm) addToInspection(toAdd string) {
	if dorm.getRoom(toAdd) == nil {
		log.Printf("Room %s not found\n", toAdd)
		return
	}

	toCheck := strings.Split(dorm.ToInspect, ", ")
	for _, roomNumber := range toCheck {
		if roomNumber == toAdd {
			log.Printf("Room %s is already on the inspection list", toAdd)
			return
		}
	}
	toCheck = append(toCheck, toAdd)
	dorm.ToInspect = strings.Join(toCheck, ", ")
	db.Model(dorm).Save(dorm)
	log.Printf("Room %s added to inspection list\n", toAdd)
}

func (dorm *Dorm) removeFromInspection(toRemove string) {
	toCheck := strings.Split(dorm.ToInspect, ", ")
	for i, roomNumber := range toCheck {
		if roomNumber == toRemove {
			toCheck = append(toCheck[:i], toCheck[i+1:]...)
			dorm.ToInspect = strings.Join(toCheck, ", ")
			db.Model(dorm).Save(dorm)
			log.Printf("Room %s removed from inspection list\n", toRemove)
			return
		}
	}
	log.Printf("Room %s is not on the inspection list\n", toRemove)
}

func (dorm *Dorm) scheduleInspection(fullyRandom bool) {
	unchecked := make([]Room, 0)
	for _, room := range dorm.rooms {
		if fullyRandom || !room.Inspected {
			unchecked = append(unchecked, room)
		}
	}

	toCheck := make([]string, 0)
	if !fullyRandom && len(unchecked) < numInspections {
		for _, room := range unchecked {
			room.Inspected = true
			room.update()
			toCheck = append(toCheck, room.RoomNumber)
		}
		unchecked = nil

		for _, room := range dorm.rooms {
			checking := false
			for _, roomToCheck := range toCheck {
				if room.RoomNumber == roomToCheck {
					checking = true
				}
			}
			if !checking {
				room.Inspected = false
				room.update()
				unchecked = append(unchecked, room)
			}
		}
	}

	rand.Seed(time.Now().Unix())
	for i := rand.Intn(len(unchecked)); len(toCheck) < numInspections; i = rand.Intn(len(unchecked)) {
		unchecked[i].Inspected = true
		unchecked[i].update()
		toCheck = append(toCheck, unchecked[i].RoomNumber)
		unchecked = append(unchecked[:i], unchecked[i+1:]...)
	}
	sort.Strings(toCheck)

	dorm.LastInspection = time.Now().UTC().Unix()
	dorm.ToInspect = strings.Join(toCheck, ", ")
	db.Model(dorm).Save(dorm)
}

func (dorm *Dorm) getInspectionList(schedule, sendToSlack bool) string {
	if schedule {
		dorm.scheduleInspection(fullyRandom)
	}

	toInspect := make([]Room, 0)
	for _, roomNumber := range strings.Split(dorm.ToInspect, ", ") {
		room := dorm.getRoom(roomNumber)
		if room != nil {
			toInspect = append(toInspect, *room)
		} else {
			log.Printf("Unable to find room %s\n", roomNumber)
		}
	}

	inspectionTime := time.Unix(dorm.LastInspection, 0)
	out := fmt.Sprintf("Room Inspections for week starting on %s", inspectionTime.Format("January 2, 2006"))
	if sendToSlack {
		out = fmt.Sprintf("*%s*", out)
	}
	out += "\n"

	for _, room := range toInspect {
		ele := room.RoomNumber + ":"
		if sendToSlack {
			ele = fmt.Sprintf(">*%s*", ele)
		}
		out += fmt.Sprintf("%s %s\n", ele, strings.Join(room.tenants, ", "))
	}

	return out
}

func (dorm *Dorm) getTenants(sendToSlack bool) string {
	out := "Nostromo Dormitory Active Rooms"
	if sendToSlack {
		out = fmt.Sprintf("*%s*", out)
	}
	out += "\n"

	for _, room := range dorm.rooms {
		ele := fmt.Sprintf("%s:", room.RoomNumber)
		if sendToSlack {
			ele = fmt.Sprintf(">*%s*", ele)
		}
		out += fmt.Sprintf("%s %s\n", ele, strings.Join(room.tenants, ", "))
	}
	return out
}

func (dorm *Dorm) getDuplicates(sendToSlack bool) string {
	out := "Tenants located in multiple rooms"
	if sendToSlack {
		out = fmt.Sprintf("*%s*", out)
	}
	out += "\n"

	duplicates := make(map[string]string)
	for _, room := range dorm.rooms {
		for _, tenant := range room.tenants {
			rooms := dorm.getRoomsForTenant(tenant)
			if len(rooms) > 1 {
				if _, present := duplicates[tenant]; !present {
					duplicates[tenant] = strings.Join(rooms, ", ")
				}
			}
		}
	}
	for tenant, rooms := range duplicates {
		ele := tenant
		if sendToSlack {
			ele = fmt.Sprintf(">*%s:*", ele)
		}
		out += fmt.Sprintf("%s %s\n", ele, rooms)
	}

	return out
}
