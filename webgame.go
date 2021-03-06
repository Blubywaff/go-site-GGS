package main

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func webgame(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(w, req) {
		if req.Method == http.MethodPost {
			_, _ = fmt.Fprint(w, "YOU ARE NOT LOGGED IN! REQUEST FAILED!")
			return
		}
		http.Redirect(w, req, "/login/", http.StatusSeeOther)
		return
	}

	if req.Method == http.MethodPost {
		webgameAjax(w, req)
		return
	}

	username := getUser(w, req).Username
	if !containsPlayer(bson.D{{"Username", username}}) {
		_ = tpls.ExecuteTemplate(w, "newplayer.gohtml", nil)
		return
	}
	if !containsPlayer(bson.D{{"Username", username}, {"HasTrained", true}}) {
		_ = tpls.ExecuteTemplate(w, "gamestart.gohtml", nil)
		return
	}
	_ = tpls.ExecuteTemplate(w, "webgame.gohtml", nil)
	//playerT := readPlayer(bson.D{{"Username", username}, {"IsTraining", true}})
	//player := readPlayer(bson.D{{"Username", username}, {"IsTraining", false}})

}

func training(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(w, req) {
		http.Redirect(w, req, "/login/", http.StatusSeeOther)
		return
	}
	//TODO SHOULD USE INIT AJAX NOT THIS
	username := getUser(w, req).Username
	writePlayer(Player{false, username, []Ship{}, Base{}})
	_ = tpls.ExecuteTemplate(w, "trainingground.gohtml", nil)
}

func gamestart(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(w, req) {
		http.Redirect(w, req, "/login/", http.StatusSeeOther)
		return
	}
	_ = tpls.ExecuteTemplate(w, "gamestart.gohtml", nil)
}

func gamedetails(w http.ResponseWriter, req *http.Request) {
	_ = tpls.ExecuteTemplate(w, "gamedetails.gohtml", nil)
}

func gameraid(w http.ResponseWriter, req *http.Request) {
	if !alreadyLoggedIn(w, req) {
		http.Redirect(w, req, "/login/", http.StatusSeeOther)
		return
	}
	_ = tpls.ExecuteTemplate(w, "gameraidselect.gohtml", nil)
}

func webgameAjax(w http.ResponseWriter, req *http.Request) {
	bytes, err := ioutil.ReadAll(req.Body)
	check(err)
	username := getUser(w, req).Username
	player := getPlayer(username)
	requests := strings.Split(string(bytes), "|")
	act := requests[0]
	var resultText string
	if act == "get" {
		data, _ := json.Marshal(getPlayer(username))
		_, _ = fmt.Fprint(w, string(data))
		return
	} else if act == "init" {
		if containsPlayer(bson.D{{"Username", username}}) {
			_, _ = fmt.Fprint(w, "error-exists")
			return
		}
		writePlayer(Player{false, username, []Ship{}, Base{Owner: username, Planets: []Planet{{1, time.Now()}, {1, time.Now()}, {1, time.Now()}, {1, time.Now()}}, Turrets: []Turret{}}})
	} else if act == "real" {
		if !containsPlayer(bson.D{{"Username", username}}) {
			_, _ = fmt.Fprint(w, "error-exists")
			return
		}
		if containsPlayer(bson.D{{"Username", username}, {"HasTrained", true}}) {
			_, _ = fmt.Fprint(w, "error-exists")
			return
		}
		updatePlayer(bson.D{{"Username", username}}, bson.D{{"$set", bson.D{{"HasTrained", true}}}})
	} else if act == "turret" {
		act = requests[1]
		base := getBase(username)
		if act == "add" {
			turret := Turret{}
			jsonout, _ := strconv.Unquote(requests[2])
			if jsonout == "" {
				jsonout = requests[2]
			}
			err = json.Unmarshal([]byte(jsonout), &turret)
			if !check(err) {
				if !json.Valid([]byte(requests[2])) {
					_, _ = fmt.Fprint(w, "error-decode-invalid")
					return
				}
				_, _ = fmt.Fprint(w, "error-decode")
				return
			}
			if base.hasTurretByPosition(turret.Position) {
				_, _ = fmt.Fprint(w, "error-exists")
				return
			}
			expense := turretAddCost(base)
			affords := canAfford(base, expense)
			if !affords[0] {
				_, _ = fmt.Fprint(w, "error-water")
				return
			} else if !affords[1] {
				_, _ = fmt.Fprint(w, "error-metal")
				return
			} else if !affords[2] {
				_, _ = fmt.Fprint(w, "error-fuel")
				return
			} else if !affords[3] {
				_, _ = fmt.Fprint(w, "error-power")
				return
			}
			doCosts(base, expense)
			id := uuid.New()
			turret.ID = id.String()
			turret.Level = 1
			writeTurret(username, turret)
		} else if act == "remove" {
			turret := Turret{}
			jsonout, _ := strconv.Unquote(requests[2])
			if jsonout == "" {
				jsonout = requests[2]
			}
			err = json.Unmarshal([]byte(jsonout), &turret)
			if !check(err) {
				_, _ = fmt.Fprint(w, "error-decode")
				return
			}
			if !base.hasTurretByID(turret.ID) {
				_, _ = fmt.Fprint(w, "error-exists")
				return
			}
			removeTurret(username, turret.ID)
		} else if act == "change" {
			turret := Turret{}
			jsonout, _ := strconv.Unquote(requests[2])
			if jsonout == "" {
				jsonout = requests[2]
			}
			err = json.Unmarshal([]byte(jsonout), &turret)
			if !check(err) {
				_, _ = fmt.Fprint(w, "error-decode")
				return
			}
			if !base.hasTurretByID(turret.ID) {
				_, _ = fmt.Fprint(w, "error-exists")
				return
			}
			original := base.getTurretByID(turret.ID)
			if original == turret {
				_, _ = fmt.Fprint(w, "error-same")
				return
			}
			if turret.Position != original.Position && turret.Level != original.Level {
				_, _ = fmt.Fprint(w, "error-multiple")
				return
			} else if turret.Position != original.Position {
				curpostur := base.getTurretByPosition(turret.Position)
				if curpostur != (Turret{}) {
					updateTurret(username, curpostur.ID, bson.D{{"$set", bson.D{{"Position", original.Position}}}})
				}
				updateTurret(username, turret.ID, bson.D{{"$set", bson.D{{"Position", turret.Position}}}})

			} else if turret.Level != original.Level {
				expense := turretLevelCost(original)
				affords := canAfford(base, expense)
				if !affords[0] {
					_, _ = fmt.Fprint(w, "error-water")
					return
				} else if !affords[1] {
					_, _ = fmt.Fprint(w, "error-metal")
					return
				} else if !affords[2] {
					_, _ = fmt.Fprint(w, "error-fuel")
					return
				} else if !affords[3] {
					_, _ = fmt.Fprint(w, "error-power")
					return
				}
				doCosts(base, expense)
				updateTurret(username, turret.ID, bson.D{{"$inc", bson.D{{"Base.Turrets.$.Level", 1}}}})
			}
		} else {
			_, _ = fmt.Fprint(w, "error-request-1")
			return
		}

	} else if act == "ship" {
		act = requests[1]
		base := getBase(username)
		if act == "add" {
			ship := Ship{}
			err = json.Unmarshal([]byte(requests[2]), &ship)
			if !check(err) {
				_, _ = fmt.Fprint(w, "error-decode")
				return
			}
			expense := shipAddCost(player)
			affords := canAfford(base, expense)
			if !affords[0] {
				_, _ = fmt.Fprint(w, "error-water")
				return
			} else if !affords[1] {
				_, _ = fmt.Fprint(w, "error-metal")
				return
			} else if !affords[2] {
				_, _ = fmt.Fprint(w, "error-fuel")
				return
			} else if !affords[3] {
				_, _ = fmt.Fprint(w, "error-power")
				return
			}
			doCosts(base, expense)
			id := uuid.New().String()
			ship.ID = id
			ship.Level = 1
			writeShip(username, ship)

		} else if act == "change" {
			ship := Ship{}
			jsonout, _ := strconv.Unquote(requests[2])
			if jsonout == "" {
				jsonout = requests[2]
			}
			err = json.Unmarshal([]byte(jsonout), &ship)
			if !check(err) {
				_, _ = fmt.Fprint(w, "error-decode")
				return
			}
			if !player.hasShipByID(ship.ID) {
				_, _ = fmt.Fprint(w, "error-exists")
				return
			}
			original := player.getShipByID(ship.ID)
			if original == ship {
				_, _ = fmt.Fprint(w, "error-same")
				return
			}
			if ship.Level != original.Level {
				expense := shipLevelCost(original)
				affords := canAfford(base, expense)
				if !affords[0] {
					_, _ = fmt.Fprint(w, "error-water")
					return
				} else if !affords[1] {
					_, _ = fmt.Fprint(w, "error-metal")
					return
				} else if !affords[2] {
					_, _ = fmt.Fprint(w, "error-fuel")
					return
				} else if !affords[3] {
					_, _ = fmt.Fprint(w, "error-power")
					return
				}
				doCosts(base, expense)
				updateShip(username, ship.ID, bson.D{{"$inc", bson.D{{"Ships.$.Level", 1}}}})
			}
		} else {
			_, _ = fmt.Fprint(w, "error-request-1")
			return
		}
	} else if act == "planet" {
		act = requests[1]
		base := getBase(username)
		//fmt.Println("Planet " + strings.Join(requests, "|"))
		if act == "thor" {
			// Power
			act = requests[2]
			if act == "level" {
				expense := planetLevelCost(base, 3)
				affords := canAfford(base, expense)
				if !affords[0] {
					_, _ = fmt.Fprint(w, "error-water")
					return
				} else if !affords[1] {
					_, _ = fmt.Fprint(w, "error-metal")
					return
				} else if !affords[2] {
					_, _ = fmt.Fprint(w, "error-fuel")
					return
				} else if !affords[3] {
					_, _ = fmt.Fprint(w, "error-power")
					return
				}
				doCosts(base, expense)
				updateBase(username, bson.D{{"$inc", bson.D{{"Base.Planets.3.Level", 1}}}})
			} else if act == "collect" {
				updateBase(username, bson.D{{"$inc", bson.D{{"Base.Power", planetCollect(base.Planets[3])}}}})
				updateBase(username, bson.D{{"$set", bson.D{{"Base.Planets.3.CollectTime", time.Now()}}}})
			} else {
				_, _ = fmt.Fprint(w, "error-request-2")
				return
			}
		} else if act == "neptune" {
			// Water
			act = requests[2]
			//fmt.Println(requests)
			if act == "level" {
				expense := planetLevelCost(base, 0)
				//fmt.Println(expense)
				affords := canAfford(base, expense)
				//fmt.Println(affords)
				if !affords[0] {
					_, _ = fmt.Fprint(w, "error-water")
					return
				} else if !affords[1] {
					_, _ = fmt.Fprint(w, "error-metal")
					return
				} else if !affords[2] {
					_, _ = fmt.Fprint(w, "error-fuel")
					return
				} else if !affords[3] {
					_, _ = fmt.Fprint(w, "error-power")
					return
				}
				doCosts(base, expense)
				updateBase(username, bson.D{{"$inc", bson.D{{"Base.Planets.0.Level", 1}}}})
			} else if act == "collect" {
				updateBase(username, bson.D{{"$inc", bson.D{{"Base.Water", planetCollect(base.Planets[0])}}}})
				updateBase(username, bson.D{{"$set", bson.D{{"Base.Planets.0.CollectTime", time.Now()}}}})
			} else {
				_, _ = fmt.Fprint(w, "error-request-2")
				return
			}
		} else if act == "titan" {
			// Metal
			act = requests[2]
			if act == "level" {
				expense := planetLevelCost(base, 1)
				affords := canAfford(base, expense)
				if !affords[0] {
					_, _ = fmt.Fprint(w, "error-water")
					return
				} else if !affords[1] {
					_, _ = fmt.Fprint(w, "error-metal")
					return
				} else if !affords[2] {
					_, _ = fmt.Fprint(w, "error-fuel")
					return
				} else if !affords[3] {
					_, _ = fmt.Fprint(w, "error-power")
					return
				}
				doCosts(base, expense)
				updateBase(username, bson.D{{"$inc", bson.D{{"Base.Planets.1.Level", 1}}}})
			} else if act == "collect" {
				updateBase(username, bson.D{{"$inc", bson.D{{"Base.Metal", planetCollect(base.Planets[1])}}}})
				updateBase(username, bson.D{{"$set", bson.D{{"Base.Planets.1.CollectTime", time.Now()}}}})
			} else {
				_, _ = fmt.Fprint(w, "error-request-2")
				return
			}
		} else if act == "helios" {
			// Fuel
			act = requests[2]
			if act == "level" {
				expense := planetLevelCost(base, 2)
				affords := canAfford(base, expense)
				if !affords[0] {
					_, _ = fmt.Fprint(w, "error-water")
					return
				} else if !affords[1] {
					_, _ = fmt.Fprint(w, "error-metal")
					return
				} else if !affords[2] {
					_, _ = fmt.Fprint(w, "error-fuel")
					return
				} else if !affords[3] {
					_, _ = fmt.Fprint(w, "error-power")
					return
				}
				doCosts(base, expense)
				updateBase(username, bson.D{{"$inc", bson.D{{"Base.Planets.2.Level", 1}}}})
			} else if act == "collect" {
				updateBase(username, bson.D{{"$inc", bson.D{{"Base.Fuel", planetCollect(base.Planets[2])}}}})
				updateBase(username, bson.D{{"$set", bson.D{{"Base.Planets.2.CollectTime", time.Now()}}}})
			} else {
				_, _ = fmt.Fprint(w, "error-request-2")
				return
			}
		} else {
			_, _ = fmt.Fprint(w, "error-request-1")
			return
		}
	} else if act == "raid" {
		act = requests[1]
		if act == "find" {
			bases := getBasesOfStrength(player.Base.Strength, 10, 3, username)
			res, err := json.Marshal(bases)
			if !check(err) {
				_, _ = fmt.Fprint(w, "error-internal")
				return
			}
			resultText = string(res)
			fmt.Println(bases)
			fmt.Println(res)
		} else if act == "do" {
			raid := Raid{}
			jsonout, _ := strconv.Unquote(requests[2])
			if jsonout == "" {
				jsonout = requests[2]
			}
			if !strings.Contains(jsonout, "\"Fleet\":[]") {
				fmt.Println("before", jsonout)
				fleetIndex := strings.Index(jsonout, "Fleet")
				fleetStart := fleetIndex + 8
				endFleet := strings.Index(jsonout[fleetStart:], "]")
				fmt.Println(fleetIndex, fleetStart, endFleet, len(jsonout))
				fmt.Println(jsonout[fleetStart : fleetStart+endFleet])
				ids := strings.Split(strings.ReplaceAll(jsonout[fleetStart:fleetStart+endFleet], "\"", ""), ",")
				fmt.Println(ids)
				fleet := []Ship{}
				for _, id := range ids {
					fleet = append(fleet, getShip(username, id))
				}
				for _, ship := range fleet {
					bb, err := json.Marshal(ship)
					check(err)
					jsonout = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(jsonout, ship.ID, string(bb)), "\"{", "{"), "}\"", "}")
				}
				fmt.Println("after", jsonout)
			}
			err = json.Unmarshal([]byte(jsonout), &raid)
			if !check(err) {
				_, _ = fmt.Fprint(w, "error-decode")
				return
			}
			result := simulateRaid(raid)
			doCosts(player.Base, []int{-result, -result, -result, -result})
			resultText = strconv.Itoa(result)
			fmt.Println(result)
		} else {
			_, _ = fmt.Fprint(w, "error-request-1")
			return
		}
	} else {
		_, _ = fmt.Fprint(w, "error-request-0")
		return
	}

	if requests[0] == "turret" || requests[0] == "planet" {
		updateBase(username, bson.D{{"$set", bson.D{{"Base.Strength", calculateStrength(player.Base)}}}})
	}

	if resultText != "" {
		_, _ = fmt.Fprint(w, "done"+"\n"+resultText)
		return
	}
	_, _ = fmt.Fprint(w, "done")
}

// Water Metal Fuel Power
func canAfford(base Base, costs []int) []bool {
	return []bool{base.Water >= costs[0], base.Metal >= costs[1], base.Fuel >= costs[2], base.Power >= costs[3]}
}

func doCosts(base Base, costs []int) {
	updateBase(base.Owner, bson.D{{"$inc", bson.D{{"Base.Water", -costs[0]}, {"Base.Metal", -costs[1]}, {"Base.Fuel", -costs[2]}, {"Base.Power", -costs[3]}}}})
}

func simulateRaid(raid Raid) int {
	attackPower := 0
	for _, ship := range raid.Fleet {
		attackPower += ship.Level
	}
	defenderPower := getBase(raid.Target).Strength
	remainder := attackPower - defenderPower
	if remainder <= 0 {
		return 0
	}
	return remainder
}

func calculateStrength(base Base) int {
	strength := 0
	for _, turret := range base.Turrets {
		strength += turret.Level
	}
	return strength
}

// Formula: #turrets^2 * turretMultiplier * turretBaseCost
func turretAddCost(base Base) []int {
	multiplier := 1
	baseCost := 1
	costs := []int{0, 0, 0, 0}
	costs[1] = int(math.Pow(float64(len(base.Turrets)), float64(2))) * multiplier * baseCost
	return costs
}

// Formula: level^2 * turretMultiplier * turretBaseCost
// Formula: level^1.2 * turretMultiplier * turretBaseCost
func turretLevelCost(turret Turret) []int {
	multiplier := 1
	baseCost := 1
	costs := []int{0, 0, 0, 0}
	costs[1] = int(math.Pow(float64(turret.Level), float64(2))) * multiplier * baseCost
	costs[2] = int(math.Pow(float64(turret.Level), 1.2)) * multiplier * baseCost
	return costs
}

// Formula: #ships^2 * shipMultiplier * shipBaseCost
func shipAddCost(player Player) []int {
	multiplier := 1
	baseCost := 1
	costs := []int{0, 0, 0, 0}
	costs[1] = int(math.Pow(float64(len(player.Ships)), float64(2))) * multiplier * baseCost
	return costs
}

// Formula: level^2 * shipMultiplier * shipBaseCost
// Formula: level^1.2 * shipMultiplier * shipBaseCost
func shipLevelCost(ship Ship) []int {
	multiplier := 1
	baseCost := 1
	costs := []int{0, 0, 0, 0}
	costs[1] = int(math.Pow(float64(ship.Level), float64(2))) * multiplier * baseCost
	costs[2] = int(math.Pow(float64(ship.Level), float64(1.2))) * multiplier * baseCost
	fmt.Println(costs)
	return costs
}

func planetLevelCost(base Base, planet int) []int {
	cost := int(math.Pow(float64(base.Planets[planet].Level), 2.0))
	return []int{cost, cost, cost, cost}
}

func planetCollect(planet Planet) int {
	return planet.Level * int(time.Now().Sub(planet.CollectTime).Seconds())
}
