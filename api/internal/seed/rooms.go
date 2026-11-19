package seed

import (
	"context"
	"fmt"
	"time"

	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/rooms"

	"github.com/google/uuid"
)

// seedWeeks is how many weeks of the sample timetable are created, starting from the current week.
const seedWeeks = 16

type roomSeed struct {
	Code       string
	Name       string
	Type       dbgen.RoomType
	Capacity   int32
	Facilities []string
}

var roomSeeds = []roomSeed{
	{"KLS-101", "Ruang Kelas 101", dbgen.RoomTypeKELAS, 40, []string{"Proyektor", "AC", "Whiteboard"}},
	{"KLS-102", "Ruang Kelas 102", dbgen.RoomTypeKELAS, 40, []string{"Proyektor", "AC", "Whiteboard"}},
	{"KLS-201", "Ruang Kelas 201", dbgen.RoomTypeKELAS, 60, []string{"Proyektor", "AC", "Sound System"}},
	{"LAB-1", "Lab Jaringan", dbgen.RoomTypeLAB, 30, []string{"Proyektor", "AC", "Switch & Router", "30 PC"}},
	{"LAB-2", "Lab RPL", dbgen.RoomTypeLAB, 40, []string{"Proyektor", "AC", "40 PC"}},
	{"LAB-3", "Lab Multimedia", dbgen.RoomTypeLAB, 35, []string{"Proyektor", "AC", "35 PC", "Tablet Grafis"}},
	{"AUD-1", "Auditorium", dbgen.RoomTypeAUDITORIUM, 300, []string{"Proyektor", "AC", "Sound System", "Panggung"}},
	{"UJI-1", "Ruang Ujian 1", dbgen.RoomTypeRUANGUJIAN, 40, []string{"AC", "Whiteboard"}},
	{"UJI-2", "Ruang Ujian 2", dbgen.RoomTypeRUANGUJIAN, 60, []string{"AC", "Whiteboard"}},
	{"SDG-1", "Ruang Sidang", dbgen.RoomTypeRUANGSIDANG, 15, []string{"Proyektor", "AC", "Meja Rapat"}},
}

type classSeed struct {
	Room    string
	Weekday time.Weekday
	Start   string // HH:MM, WIB
	End     string
	Title   string
}

var timetable = []classSeed{
	{"LAB-2", time.Monday, "07:00", "09:30", "Basis Data (Praktikum)"},
	{"KLS-101", time.Monday, "08:00", "10:00", "Algoritma dan Pemrograman"},
	{"LAB-1", time.Monday, "13:00", "15:30", "Jaringan Komputer (Praktikum)"},
	{"KLS-102", time.Tuesday, "09:00", "11:00", "Matematika Diskrit"},
	{"LAB-3", time.Tuesday, "10:00", "12:30", "Multimedia (Praktikum)"},
	{"KLS-201", time.Tuesday, "13:00", "15:00", "Manajemen Proyek TI"},
	{"LAB-2", time.Wednesday, "09:00", "11:30", "Pemrograman Web (Praktikum)"},
	{"KLS-101", time.Wednesday, "10:00", "12:00", "Sistem Operasi"},
	{"LAB-1", time.Thursday, "08:00", "10:30", "Keamanan Jaringan (Praktikum)"},
	{"KLS-201", time.Thursday, "10:00", "12:00", "Rekayasa Perangkat Lunak"},
	{"KLS-102", time.Friday, "07:30", "09:30", "Basis Data (Teori)"},
	{"LAB-3", time.Friday, "13:00", "15:30", "Desain Antarmuka (Praktikum)"},
}

// seedRooms creates the sample rooms and a weekly timetable. Both steps skip what already exists.
func seedRooms(ctx context.Context, q *dbgen.Queries, unitID, createdBy uuid.UUID, now time.Time) (newRooms, newSchedules int, err error) {
	for _, r := range roomSeeds {
		token, err := rooms.NewQRToken()
		if err != nil {
			return newRooms, newSchedules, err
		}
		n, err := q.SeedRoom(ctx, dbgen.SeedRoomParams{
			OrgUnitID: unitID, Code: r.Code, Name: r.Name, Type: r.Type,
			Capacity: r.Capacity, Facilities: r.Facilities, QrToken: token, QrRotatedAt: now,
		})
		if err != nil {
			return newRooms, newSchedules, fmt.Errorf("seeding room %s: %w", r.Code, err)
		}
		newRooms += int(n)
	}

	ids := make(map[string]uuid.UUID, len(roomSeeds))
	for _, r := range roomSeeds {
		row, err := q.GetRoomByCode(ctx, dbgen.GetRoomByCodeParams{OrgUnitID: unitID, Code: r.Code})
		if err != nil {
			return newRooms, newSchedules, fmt.Errorf("looking up room %s: %w", r.Code, err)
		}
		ids[r.Code] = row.ID
	}

	// Monday of the current week in WIB.
	today := clock.ToWIB(now)
	weekStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, clock.WIB)
	weekStart = weekStart.AddDate(0, 0, -((int(weekStart.Weekday()) + 6) % 7))

	for week := 0; week < seedWeeks; week++ {
		for _, c := range timetable {
			day := weekStart.AddDate(0, 0, 7*week+(int(c.Weekday)+6)%7)
			start, err := atTime(day, c.Start)
			if err != nil {
				return newRooms, newSchedules, err
			}
			end, err := atTime(day, c.End)
			if err != nil {
				return newRooms, newSchedules, err
			}
			n, err := q.SeedSchedule(ctx, dbgen.SeedScheduleParams{
				RoomID: ids[c.Room], Title: c.Title, StartAt: start.UTC(), EndAt: end.UTC(),
				CreatedBy: createdBy, CreatedAt: now,
			})
			if err != nil {
				return newRooms, newSchedules, fmt.Errorf("seeding schedule %s: %w", c.Title, err)
			}
			newSchedules += int(n)
		}
	}
	return newRooms, newSchedules, nil
}

func atTime(day time.Time, hhmm string) (time.Time, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing seed time %q: %w", hhmm, err)
	}
	return time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), 0, 0, clock.WIB), nil
}
