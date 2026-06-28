package assets

import "github.com/Zyko0/EbitengineJam2026/core/level"

// Rooms is the authored building-block library 
var Rooms = []level.Room{
	// Two opposite doors: corridors and simple square rooms
	{
		Name: "corr_wide", Edges: level.EN | level.ES, Weight: 3,
		Tags:    level.TagCorridor | level.TagLongLane,
		Carve:   func(s *level.RoomStamp) { s.OpenRect(5, 0, 10, 15) }, // 6-wide lane
	},
	{
		Name: "corr_choke", Edges: level.EN | level.ES, Weight: 2,
		Tags:    level.TagCorridor | level.TagChokepoint,
		Carve: func(s *level.RoomStamp) {
			s.OpenRect(5, 0, 10, 15)
			for z := 6; z <= 9; z++ { // pinch the middle to a two-file lane
				s.Block(5, z)
				s.Block(6, z)
				s.Block(9, z)
				s.Block(10, z)
			}
		},
	},
	{
		Name: "hall_open2", Edges: level.EN | level.ES, Weight: 3,
		Tags:   level.TagHall,
		Carve:  func(s *level.RoomStamp) { s.OpenRect(1, 1, 14, 14) },
	},

	// Two adjacent doors: bends and corner rooms
	{
		Name: "bend_wide", Edges: level.EN | level.EE, Weight: 4,
		Tags:   level.TagCorridor,
		Carve: func(s *level.RoomStamp) {
			s.OpenRect(5, 0, 10, 10) // vertical leg from N
			s.OpenRect(5, 5, 15, 10) // horizontal leg to E
		},
	},
	{
		Name: "room_corner", Edges: level.EN | level.EE, Weight: 2,
		Tags:   level.TagHall,
		Carve:  func(s *level.RoomStamp) { s.OpenRect(1, 1, 14, 14) },
	},

	// Three doors: corridor T (tight) or open hall (rare)
	{
		Name: "junc_T", Edges: level.EN | level.EE | level.ES, Weight: 4,
		Tags:    level.TagCorridor | level.TagJunction,
		Carve: func(s *level.RoomStamp) {
			s.OpenRect(5, 0, 10, 15) // N-S lane
			s.OpenRect(5, 5, 15, 10) // E lane into the centre
		},
	},
	{
		Name: "hall_open3", Edges: level.EN | level.EE | level.ES, Weight: 2,
		Tags:    level.TagHall | level.TagJunction,
		Carve:   func(s *level.RoomStamp) { s.OpenRect(1, 1, 14, 14) },
	},

	// Four doors: corridor cross (tight) or open hall (rare)
	{
		Name: "junc_cross", Edges: level.EN | level.EE | level.ES | level.EW, Weight: 4,
		Tags:   level.TagCorridor | level.TagJunction,
		Carve: func(s *level.RoomStamp) {
			s.OpenRect(5, 0, 10, 15) // N-S lane
			s.OpenRect(0, 5, 15, 10) // E-W lane
		},
	},
	{
		Name: "hall_open4", Edges: level.EN | level.EE | level.ES | level.EW, Weight: 2,
		Tags:    level.TagHall | level.TagJunction,
		Carve:   func(s *level.RoomStamp) { s.OpenRect(1, 1, 14, 14) },
	},

	// Terminals and specials (one door)
	{
		Name: "room_end", Edges: level.EN, Weight: 3,
		Tags:   level.TagDeadEnd,
		Carve:  func(s *level.RoomStamp) { s.OpenRect(1, 1, 14, 14) }, // payoff is the event
	},
	{
		Name: "room_spawn", Edges: level.EN, Weight: 1,
		Tags:  level.TagSpawn | level.TagDeadEnd,
		Carve: func(s *level.RoomStamp) { s.OpenRect(1, 1, 14, 14) },
	},
	{
		Name: "room_elevator", Edges: level.EN, Weight: 1,
		Tags:    level.TagElevator,
		Carve:   func(s *level.RoomStamp) { s.OpenRect(1, 1, 14, 14) },
	},
}
