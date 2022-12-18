package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"
	"strconv"
	// "strings"
)

type Passenger struct {
	PassengerId  int    `json:"passengerId"`
	FirstName    string `json:"firstName"`
	LastName     string `json:"lastName"`
	MobileNumber string `json:"mobileNumber"`
	EmailAddr    string `json:"emailAddr"`
	Password     string `json:"password"`
}

type Driver struct {
	DriverId      int    `json:"driverId"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	MobileNumber  string `json:"mobileNumber"`
	EmailAddr     string `json:"emailAddr"`
	Password      string `json:"password"`
	LicenseNumber string `json:"licenseNumber"`
	IdNumber      string `json:"idNumber"`
	DriverStatus  string `json:"driverStatus"`
}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/trip", tripEndpoint).Methods("GET", "POST")
	router.HandleFunc("/driver/trip", driverTripEndpoint).Methods("GET", "PATCH")
	fmt.Println("Listening at port 5001")
	log.Fatal(http.ListenAndServe(":5001", router))
}

func tripEndpoint(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		type TripDetails struct {
			PassengerId int `json:"passengerId"`
		}
		//Digest DriverAuth object from Body
		var td TripDetails
		err := json.NewDecoder(r.Body).Decode(&td)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if td.PassengerId == 0 {
			http.Error(w, "Data missing", http.StatusBadRequest)
			return
		}

		//Init DB Connection
		db, err := sql.Open("mysql", "root:password@tcp(127.0.0.1:3306)/etiassignone?parseTime=true")
		if err != nil {
			panic(err.Error())
		}
		//Select Trips from DB
		query := fmt.Sprintf("SELECT * FROM trip WHERE passengerId=%d ORDER BY tripId DESC;", td.PassengerId)
		results, err := db.Query(query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		type Trip struct {
			TripId            int       `json:"tripId"`
			PickUpPostalCode  string    `json:"pickUpPostalCode"`
			DropOffPostalCode string    `json:"dropOffPostalCode"`
			PassengerId       int       `json:"passengerId"`
			StartTime         sql.NullTime `json:"startTime"`
			EndTime           sql.NullTime `json:"endTime"`
			TripStatus        string    `json:"tripStatus"`
			RequestTime       sql.NullTime `json:"requestTime"`
			DriverId	int	`json:"driverId"`
		}
		tripMap := make(map[int]Trip)
		for results.Next() {
			var t Trip
			err = results.Scan(&t.TripId, &t.PickUpPostalCode, &t.DropOffPostalCode, &t.PassengerId, &t.StartTime, &t.EndTime, &t.TripStatus, &t.RequestTime,&t.DriverId)
			// if err != nil {
			// 	if !t.StartTime.valid || !t.EndTime.valid {
			// 		http.Error(w, err.Error(), http.StatusBadRequest)
			// 		return
			// 	}
			// }
			tripMap[t.TripId] = t
		}
		output, _ := json.Marshal(map[string]interface{}{"Trips": tripMap})
		fmt.Fprintf(w, string(output))
		defer db.Close()
	case "POST": //Tested
		//Digest TripRequest object from Body
		type TripRequest struct {
			PickUpPostalCode  string `json:"pickUpPostalCode"`
			DropOffPostalCode string `json:"dropOffPostalCode"`
			PassengerId       int    `json:"passengerId"`
			RequestTime       string `json:"requestTime"`
		}
		var tr TripRequest
		err := json.NewDecoder(r.Body).Decode(&tr)
		timeInput, _ := time.Parse(time.RFC3339, tr.RequestTime)
		mysqlTimeInput := timeInput.Format("2006-01-02 15:04:05")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		//Init DB Connection
		db, err := sql.Open("mysql", "root:password@tcp(127.0.0.1:3306)/etiassignone")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		//Check for existing ride
		checkExistingRideQuery := fmt.Sprintf("SELECT tripId FROM trip WHERE (tripStatus='pending' OR tripStatus='Ongoing') AND passengerId=%d;",tr.PassengerId)
		var exitingRideId int
		db.QueryRow(checkExistingRideQuery).Scan(&exitingRideId)
		if exitingRideId != 0 {
			http.Error(w, "Existing pending ride", http.StatusConflict)
			return
		}
		//Check for available driver
		checkQuery := "SELECT driverId FROM Driver WHERE driverStatus='Available';"
		var id int
		err = db.QueryRow(checkQuery).Scan(&id)
		if err != nil {
			http.Error(w, "No driver available", http.StatusBadRequest)
			return
		}
		//Insert Trip into DB
		query := fmt.Sprintf("INSERT INTO trip (pickUpPostalCode, dropOffPostalCode, passengerId, tripStatus, requestTime,driverId) VALUES ('%s', '%s', %d, '%s', '%s',%d);", tr.PickUpPostalCode, tr.DropOffPostalCode, tr.PassengerId, "pending", mysqlTimeInput,id)
		insert, err := db.Query(query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		insert.Close()
		//Update Driver status to Busy in DB
		statusUpdateQuery := fmt.Sprintf("UPDATE driver SET driverStatus='Hired' WHERE driverId =%d;",id)
		update, err := db.Query(statusUpdateQuery)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		update.Close()
		w.WriteHeader(http.StatusAccepted)
		defer db.Close()
	default:
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
}

func driverTripEndpoint(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		type TripDetails struct {
			DriverId int `json:"driverId"`
		}
		//Digest DriverAuth object from Body
		var td TripDetails
		err := json.NewDecoder(r.Body).Decode(&td)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if td.DriverId == 0 {
			http.Error(w, "Data missing", http.StatusBadRequest)
			return
		}

		//Init DB Connection
		db, err := sql.Open("mysql", "root:password@tcp(127.0.0.1:3306)/etiassignone?parseTime=true")
		if err != nil {
			panic(err.Error())
		}
		//Select Trips from DB
		query := fmt.Sprintf("SELECT * FROM trip WHERE driverId=%d ORDER BY tripId DESC;", td.DriverId)
		results, err := db.Query(query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		type Trip struct {
			TripId            int       `json:"tripId"`
			PickUpPostalCode  string    `json:"pickUpPostalCode"`
			DropOffPostalCode string    `json:"dropOffPostalCode"`
			PassengerId       int       `json:"passengerId"`
			StartTime         sql.NullTime `json:"startTime"`
			EndTime           sql.NullTime `json:"endTime"`
			TripStatus        string    `json:"tripStatus"`
			RequestTime       sql.NullTime `json:"requestTime"`
			DriverId	int	`json:"driverId"`
		}
		tripMap := make(map[int]Trip)
		for results.Next() {
			var t Trip
			err = results.Scan(&t.TripId, &t.PickUpPostalCode, &t.DropOffPostalCode, &t.PassengerId, &t.StartTime, &t.EndTime, &t.TripStatus, &t.RequestTime,&t.DriverId)
			// if err != nil {
			// 	if !t.StartTime.valid || !t.EndTime.valid {
			// 		http.Error(w, err.Error(), http.StatusBadRequest)
			// 		return
			// 	}
			// }
			tripMap[t.TripId] = t
		}
		output, _ := json.Marshal(map[string]interface{}{"Trips": tripMap})
		fmt.Fprintf(w, string(output))
		defer db.Close()
	case "PATCH": //Tested
		//Digest TripUpdate object from Body
		type TripUpdate struct {
			TripId            int       `json:"tripId"`
			StartTime         string `json:"startTime"`
			EndTime           string `json:"endTime"`
			TripStatus        string    `json:"tripStatus"`
			DriverId string `json:"driverId"`
		}
		var tr TripUpdate
		err := json.NewDecoder(r.Body).Decode(&tr)
		startTimeIngest, _ := time.Parse(time.RFC3339, tr.StartTime)
		startTimeDigest := startTimeIngest.Format("2006-01-02 15:04:05")
		endTimeIngest, _ := time.Parse(time.RFC3339, tr.EndTime)
		endTimeDigest := endTimeIngest.Format("2006-01-02 15:04:05")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if(endTimeDigest=="0001-01-01 00:00:00"){
			//updating start time
	
			//Init DB Connection
			db, err := sql.Open("mysql", "root:password@tcp(127.0.0.1:3306)/etiassignone")
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			tripUpdateQuery := fmt.Sprintf("UPDATE trip SET startTime='%s',tripStatus='%s' WHERE tripId =%d;",startTimeDigest,tr.TripStatus,tr.TripId)
			update, err := db.Query(tripUpdateQuery)
			if err != nil {	
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			update.Close()
			//Update Driver status to Busy in DB
			statusUpdateQuery := fmt.Sprintf("UPDATE driver SET driverStatus='Busy' WHERE driverId ="+tr.DriverId+";")
			driverUpdate, err := db.Query(statusUpdateQuery)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			driverUpdate.Close()
			w.WriteHeader(http.StatusAccepted)
			defer db.Close()
		}else{
			//updating end time
	
			//Init DB Connection
			db, err := sql.Open("mysql", "root:password@tcp(127.0.0.1:3306)/etiassignone")
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			tripUpdateQuery := fmt.Sprintf("UPDATE trip SET endTime='%s',tripStatus='%s' WHERE tripId =%d;",endTimeDigest,tr.TripStatus,tr.TripId)
			update, err := db.Query(tripUpdateQuery)
			if err != nil {	
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			update.Close()
			//Update Driver status to Busy in DB
			intVar, err:=strconv.Atoi(tr.DriverId)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			statusUpdateQuery := fmt.Sprintf("UPDATE driver SET driverStatus='Available' WHERE driverId =%d;",intVar)
			driverUpdate, err := db.Query(statusUpdateQuery)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			driverUpdate.Close()
			w.WriteHeader(http.StatusAccepted)
			defer db.Close()
		}

	default:
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
}