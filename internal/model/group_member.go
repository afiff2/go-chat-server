package model

import (
	"time"
)

type GroupMember struct {
	GroupUuid string `gorm:"column:group_uuid;type:char(37);not null;primaryKey;comment:群组uuid"`
	UserUuid  string `gorm:"column:user_uuid;type:char(37);not null;primaryKey;comment:用户uuid"`

	JoinedAt time.Time `gorm:"column:joined_at;type:datetime;not null;default:CURRENT_TIMESTAMP;comment:加入时间"`

	Group GroupInfo `gorm:"foreignKey:GroupUuid;references:Uuid;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	User  UserInfo  `gorm:"foreignKey:UserUuid;references:Uuid;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

func (GroupMember) TableName() string {
	return "group_member"
}
