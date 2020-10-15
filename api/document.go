/*
 * Virtualpaper is a service to manage users paper documents in virtual format.
 * Copyright (C) 2020  Tero Vierimaa
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package api

import (
	"github.com/sirupsen/logrus"
	"net/http"
	"tryffel.net/go/virtualpaper/models"
)

type documentResponse struct {
	Id        int `json:"id"`
	Name      string
	Filename  string
	Content   string
	CreatedAt PrettyTime
	UpdatedAt PrettyTime
}

func responseFromDocument(doc *models.Document) *documentResponse {
	resp := &documentResponse{
		Id:        doc.Id,
		Name:      doc.Name,
		Filename:  doc.Filename,
		Content:   doc.Content,
		CreatedAt: PrettyTime(doc.CreatedAt),
		UpdatedAt: PrettyTime(doc.UpdatedAt),
	}
	return resp
}

func (a *Api) getDocuments(resp http.ResponseWriter, req *http.Request) {
	user, ok := getUserId(req)
	if !ok {
		logrus.Errorf("no user in context")
		respInternalError(resp)
		return
	}

	paging, err := getPaging(req)
	if err != nil {
		respBadRequest(resp, err.Error(), nil)
	}

	docs, count, err := a.db.DocumentStore.GetDocuments(user, paging)
	if err != nil {
		logrus.Errorf("get documents: %v", err)
		respInternalError(resp)
		return
	}
	respDocs := make([]*documentResponse, len(*docs))

	for i, v := range *docs {
		respDocs[i] = responseFromDocument(&v)
	}

	respResourceList(resp, respDocs, count)
}

func (a *Api) getDocument(resp http.ResponseWriter, req *http.Request) {
	user, ok := getUserId(req)
	if !ok {
		logrus.Errorf("no user in context")
		respInternalError(resp)
		return
	}
	idStr := mux.Vars(req)["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		respBadRequest(resp, "id not integer", nil)
		return
	}

	doc, err := a.db.DocumentStore.GetDocument(user, id)

	respOk(resp, responseFromDocument(doc))
}

func (a *Api) getDocumentLogs(resp http.ResponseWriter, req *http.Request) {
	user, ok := getUserId(req)
	if !ok {
		logrus.Errorf("no user in context")
		respInternalError(resp)
		return
	}
	id, err := getParamId(req)
	if err != nil {
		respBadRequest(resp, err.Error(), nil)
		return
	}

	owns, err := a.db.DocumentStore.UserOwnsDocument(id, user)
	if err != nil {
		logrus.Errorf("Get document ownserhip: %v", err)
		respError(resp, err)
		return
	}

	if !owns {
		respUnauthorized(resp)
		return
	}

	job, err := a.db.JobStore.GetByDocument(id)
	if err != nil {
		logrus.Errorf("get document jobs: %v", err)
		respError(resp, err)
		return
	}
	respOk(resp, job)
}
