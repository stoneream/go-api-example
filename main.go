package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Item struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ItemDict = map[int]Item

type JsonDatabase struct {
	JsonFilePath  string
	jsonFileMutex sync.Mutex
}

func (j *JsonDatabase) Exists() bool {
	_, err := os.Stat(j.JsonFilePath)
	return err == nil
}

func (j *JsonDatabase) CreateJsonFile() error {
	j.jsonFileMutex.Lock()
	defer j.jsonFileMutex.Unlock()

	file, err := os.Create(j.JsonFilePath)
	if err != nil {
		log.Println("create err", err)
		return err
	}
	defer file.Close()

	return nil
}

func (j *JsonDatabase) ReadJsonFile() (ItemDict, error) {
	j.jsonFileMutex.Lock()
	defer j.jsonFileMutex.Unlock()

	file, err := os.Open(j.JsonFilePath)
	if err != nil {
		log.Println("open err", err)
		return nil, err
	}
	defer file.Close()

	bytedata, err := io.ReadAll(file)
	if err != nil {
		log.Println("read err", err)
		return nil, err
	}

	items := make([]Item, 0)
	itemDict := make(ItemDict)
	if err := json.Unmarshal(bytedata, &items); err != nil {
		log.Println("json unmarshal err", err)
		return nil, err
	}
	for _, item := range items {
		itemDict[item.ID] = item
	}

	return itemDict, nil
}

func (j *JsonDatabase) WriteJsonFile(items ItemDict) error {
	j.jsonFileMutex.Lock()
	defer j.jsonFileMutex.Unlock()

	bytes, _ := json.Marshal(items)
	file, err := os.Open(j.JsonFilePath)
	if err != nil {
		log.Println("open err", err)
		return err
	}
	defer file.Close()

	err = os.WriteFile(j.JsonFilePath, bytes, 0644)
	if err != nil {
		log.Println("write err", err)
		return err
	}

	return nil
}

var jsonDatabase = JsonDatabase{
	JsonFilePath:  "./db/data.json",
	jsonFileMutex: sync.Mutex{},
}

func getItem(w http.ResponseWriter, r *http.Request) {
	id, _ := extractParams(w, r)
	loadedItems, err := jsonDatabase.ReadJsonFile()

	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	} else {
		if id != loadedItems[id].ID {
			log.Println("Invalid ID", id, loadedItems[id])
			http.Error(w, "Invalid ID", http.StatusBadRequest)
		} else {
			json.NewEncoder(w).Encode(loadedItems[id])
			log.Println("Get item", loadedItems[id])
		}
	}
}

func deleteItem(w http.ResponseWriter, r *http.Request) {
	id, _ := extractParams(w, r)
	loadedItems, err := jsonDatabase.ReadJsonFile()

	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	} else {
		if id != loadedItems[id].ID {
			log.Println("Invalid ID", id, loadedItems[id])
			http.Error(w, "Invalid ID", http.StatusBadRequest)
		} else {
			delete(loadedItems, id)
			jsonDatabase.WriteJsonFile(loadedItems)
			log.Println("Deleted item", loadedItems[id])
		}
	}
}

func postItem(w http.ResponseWriter, r *http.Request) {
	id, name := extractParams(w, r)
	loadedItems, err := jsonDatabase.ReadJsonFile()

	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	} else {
		var newItem Item

		if err := json.NewDecoder(r.Body).Decode(&newItem); err != nil {
			http.Error(w, "Invalid input", http.StatusBadRequest)
		} else {
			loadedItems[newItem.ID] = Item{ID: id, Name: name}
			jsonDatabase.WriteJsonFile(loadedItems)
			log.Println("Posted item", loadedItems[id])
			json.NewEncoder(w).Encode(loadedItems[id])
		}
	}
}

func extractParams(w http.ResponseWriter, r *http.Request) (int, string) {
	path := strings.TrimPrefix(r.URL.Path, "/") // trim path
	splited := strings.Split(path, "/")         // split path
	fmt.Println(splited)                        // print splited
	id, err := strconv.Atoi(splited[1])         // transform string to int
	name := splited[2]                          // name is item

	if err != nil {
		fmt.Println("hugahuga", r.URL.Path, err, splited[1]) // print error
		http.Error(w, "Invalid ID", http.StatusBadRequest)   // invalid id

	}
	return id, name // retrun id, name
}

type Route struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

func requestRouter(responseWriter http.ResponseWriter, request *http.Request) {
	getItemRoute := Route{
		Method:  http.MethodGet,
		Path:    "/GET/",
		Handler: getItem,
	}
	postItemRoute := Route{
		Method:  http.MethodPost,
		Path:    "/POST/",
		Handler: postItem,
	}
	deleteItemRoute := Route{
		Method:  http.MethodDelete,
		Path:    "/DELETE/",
		Handler: deleteItem,
	}

	routes := []Route{getItemRoute, postItemRoute, deleteItemRoute}

	if request.Method != http.MethodGet && request.Method != http.MethodPost && request.Method != http.MethodDelete {
		http.Error(responseWriter, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	for _, route := range routes {
		if request.Method == route.Method && strings.HasPrefix(request.URL.Path, route.Path) {
			route.Handler(responseWriter, request)
			return
		}
	}

	http.Error(responseWriter, "Not found", http.StatusNotFound)
}

func main() {
	if !jsonDatabase.Exists() {
		err := jsonDatabase.CreateJsonFile()
		if err != nil {
			log.Println("json file create error", err)
			panic(err)
		}
	}

	http.HandleFunc("/", requestRouter)
	http.ListenAndServe(":8080", nil)
}
