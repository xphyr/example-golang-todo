package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"regexp"

	_ "github.com/denisenkom/go-mssqldb"
)

// net/http based router
type route struct {
	pattern *regexp.Regexp
	verb    string
	handler http.Handler
}

type RegexpHandler struct {
	routes []*route
}

func (h *RegexpHandler) Handler(pattern *regexp.Regexp, verb string, handler http.Handler) {
	h.routes = append(h.routes, &route{pattern, verb, handler})
}

func (h *RegexpHandler) HandleFunc(r string, v string, handler func(http.ResponseWriter, *http.Request)) {
	re := regexp.MustCompile(r)
	h.routes = append(h.routes, &route{re, v, http.HandlerFunc(handler)})
}

func (h *RegexpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range h.routes {
		if route.pattern.MatchString(r.URL.Path) && route.verb == r.Method {
			route.handler.ServeHTTP(w, r)
			return
		}
	}
	http.NotFound(w, r)
}

// todo "Object"
type Todo struct {
	Id       int    `json:"Id,sting"`
	Title    string `json:"Title"`
	Category string `json:"Category"`
	// Dt_created   string `json:"Dt_created"`
	// Dt_completed string `json:"Dt_completed"`
	State string `json:"State"`
}

// store "context" values and connections in the server struct
type Server struct {
	db *sql.DB
}

var (
	debug    = flag.Bool("debug", false, "enable debugging")
	password = flag.String("password", "", "the database password")
	database = flag.String("database", "", "the database name")
	port     = flag.Int("port", 1433, "the database port")
	server   = flag.String("server", "", "the database server")
	user     = flag.String("user", "", "the database user")
)

func main() {
	flag.Parse()

	if *debug {
		fmt.Printf(" password:%s\n", *password)
		fmt.Printf(" port:%d\n", *port)
		fmt.Printf(" server:%s\n", *server)
		fmt.Printf(" database:%s\n", *database)
		fmt.Printf(" user:%s\n", *user)
	}

	connString := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%d;database=%s", *server, *user, *password, *port, *database)
	if *debug {
		fmt.Printf(" connString:%s\n", connString)
	}

	db, err := sql.Open("mssql", connString)
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxIdleConns(100)
	defer db.Close()

	server := &Server{db: db}

	reHandler := new(RegexpHandler)

	reHandler.HandleFunc("/todos/$", "GET", server.todoIndex)
	reHandler.HandleFunc("/todos/$", "POST", server.todoCreate)
	reHandler.HandleFunc("/todos/[0-9]+$", "GET", server.todoShow)
	reHandler.HandleFunc("/todos/[0-9]+$", "PUT", server.todoUpdate)
	reHandler.HandleFunc("/todos/[0-9]+$", "DELETE", server.todoDelete)

	reHandler.HandleFunc(".*.[js|css|png|eof|svg|ttf|woff]", "GET", server.assets)
	reHandler.HandleFunc("/", "GET", server.homepage)

	fmt.Println("Starting server on port 3000")
	http.ListenAndServe(":3000", reHandler)
}

// simple HTML/JS/CSS pages

func (s *Server) homepage(res http.ResponseWriter, req *http.Request) {
	http.ServeFile(res, req, "./index.html")
}

func (s *Server) assets(res http.ResponseWriter, req *http.Request) {
	http.ServeFile(res, req, req.URL.Path[1:])
}

// Todo CRUD

func (s *Server) todoIndex(res http.ResponseWriter, req *http.Request) {
	var todos []*Todo

	rows, err := s.db.Query("SELECT Id, Title, Category, State FROM Todo")
	errorCheck(res, err)
	for rows.Next() {
		todo := &Todo{}
		rows.Scan(&todo.Id, &todo.Title, &todo.Category, &todo.State)
		todos = append(todos, todo)
	}
	rows.Close()

	jsonResponse(res, todos)
}

func (s *Server) todoCreate(res http.ResponseWriter, req *http.Request) {
	todo := &Todo{}

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&todo)
	if err != nil {
		fmt.Println("ERROR decoding JSON - ", err)
		return
	}
	defer req.Body.Close()

	result, err := s.db.Exec("INSERT INTO Todo(Title, Category, State) VALUES(?, ?, ?)", todo.Title, todo.Category, todo.State)
	if err != nil {
		fmt.Println("ERROR saving to db - ", err)
	}

	ID64, err := result.LastInsertId()
	ID := int(ID64)
	todo = &Todo{Id: ID}

	s.db.QueryRow("SELECT State, Title, Category FROM Todo WHERE Id=?", todo.Id).Scan(&todo.State, &todo.Title, &todo.Category)

	jsonResponse(res, todo)
}

func (s *Server) todoShow(res http.ResponseWriter, req *http.Request) {
	fmt.Println("Render todo json")
}

func (s *Server) todoUpdate(res http.ResponseWriter, req *http.Request) {
	todoParams := &Todo{}

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&todoParams)
	if err != nil {
		fmt.Println("ERROR decoding JSON - ", err)
		return
	}

	_, err = s.db.Exec("UPDATE Todo SET Title = ?, Category = ?, State = ? WHERE Id = ?", todoParams.Title, todoParams.Category, todoParams.State, todoParams.Id)

	if err != nil {
		fmt.Println("ERROR saving to db - ", err)
	}

	todo := &Todo{Id: todoParams.Id}
	err = s.db.QueryRow("SELECT State, Title, Category FROM Todo WHERE Id=?", todo.Id).Scan(&todo.State, &todo.Title, &todo.Category)
	if err != nil {
		fmt.Println("ERROR reading from db - ", err)
	}

	jsonResponse(res, todo)
}

func (s *Server) todoDelete(res http.ResponseWriter, req *http.Request) {
	r, _ := regexp.Compile(`\d+$`)
	ID := r.FindString(req.URL.Path)
	_, err := s.db.Exec("DELETE FROM Todo WHERE Id=?", ID)
	if err != nil {
		res.WriteHeader(500)
	} else {
		res.WriteHeader(200)
	}
}

func jsonResponse(res http.ResponseWriter, data interface{}) {
	res.Header().Set("Content-Type", "application/json; charset=utf-8")

	payload, err := json.Marshal(data)
	if errorCheck(res, err) {
		return
	}

	fmt.Fprint(res, string(payload))
}

func errorCheck(res http.ResponseWriter, err error) bool {
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return true
	}
	return false
}
