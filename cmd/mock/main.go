package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Request struct {
	Comments []string `json:"comments"`
}

type Response struct {
	Markdown string `json:"markdown"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	/*
		var req Request
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}*/

	resp := Response{
		Markdown: "### Краткая сводка по комментариям\n- Пользователи хвалят качество звука. Особенно отмечают басы и низкие частоты.\n- Клиентам не очень нравится треск пластика продукта. Возможно, следует заменить материал изготовления наушников",
	}
	time.Sleep(8 * time.Second)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	http.HandleFunc("/", handler)
	log.Println("Server is running on port 8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}
