package main

import "github.com/jinzhu/gorm"

type Room struct {
	gorm.Model
	RoomNumber    string   `gorm:"NOT NULL"`
	LastInspected int64    `gorm:"DEFAULT:0"`
	Inspected     bool     `gorm:"DEFAULT:false"`
	tenants       []string `gorm:"-"`
}

func (room *Room) update() {
	db.Model(room).Where("id = ?", room.ID).Save(room)
}
