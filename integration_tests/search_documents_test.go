package integrationtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
	"tryffel.net/go/virtualpaper/api"
	"tryffel.net/go/virtualpaper/models"
)

type DocumentSearchSuite struct {
	ApiTestSuite
	docs map[string]*api.DocumentResponse

	keys   map[string]*models.MetadataKey
	values map[string]map[string]*models.MetadataValue
}

func TestDocumentSearch(t *testing.T) {
	suite.Run(t, new(DocumentSearchSuite))
}

func (suite *DocumentSearchSuite) SetupTest() {
	suite.Init()
	clearDbDocumentTables(suite.T(), suite.db)
	clearMeiliIndices(suite.T())
	waitIndexingReady(suite.T(), suite.userHttp, 20)
	suite.docs = make(map[string]*api.DocumentResponse)
	suite.keys, suite.values = initMetadataKeyValues(suite.T(), suite.userHttp)

	text1Id := uploadDocument(suite.T(), suite.userClient, "jpg-1.jpeg", "Lorem ipsum", 20)
	text1 := getDocument(suite.T(), suite.userHttp, text1Id, 200)

	text2d := uploadDocument(suite.T(), suite.userClient, "pdf-1.pdf", "Lorem ipsum", 40)
	text2 := getDocument(suite.T(), suite.userHttp, text2d, 200)

	suite.docs["text-1"] = text1
	suite.docs["text-2"] = text2

	text1.Metadata = append(text1.Metadata, models.Metadata{
		KeyId:   suite.keys["author"].Id,
		ValueId: suite.values["author"]["doyle"].Id,
	},
		models.Metadata{
			KeyId:   suite.keys["category"].Id,
			ValueId: suite.values["category"]["paper"].Id,
		},
	)
	text1.Date = time.Now().UnixMilli()

	text2.Metadata = append(text1.Metadata, models.Metadata{
		KeyId:   suite.keys["author"].Id,
		ValueId: suite.values["author"]["darwin"].Id,
	},
		models.Metadata{
			KeyId:   suite.keys["category"].Id,
			ValueId: suite.values["category"]["paper"].Id,
		},
	)
	text2.Date = time.Now().AddDate(-1, 0, 0).UnixMilli()
	updateDocument(suite.T(), suite.userHttp, text1, 200)
	updateDocument(suite.T(), suite.userHttp, text2, 200)

	suite.docs["text-1"] = getDocument(suite.T(), suite.userHttp, text1.Id, 200)
	suite.docs["text-2"] = getDocument(suite.T(), suite.userHttp, text2.Id, 200)
	suite.T().Log("wait for indexing to finish")
	waitIndexingReady(suite.T(), suite.userHttp, 60)
	suite.T().Log("indexing finished")
}

func (suite *DocumentSearchSuite) TestSearchByDate() {
	// api.DocumentFilter
	filter := map[string]string{
		"q": "date:today",
		//"metadata": "",
	}

	docs := searchDocuments(suite.T(), suite.userHttp, filter, 1, 10, "name", "ASC", 200)
	//assert.Equal(suite.T(), 1, len(docs))
	assert.Equal(suite.T(), suite.docs["text-1"].Id, docs[0].Id)

	// this year
	q := fmt.Sprintf("date:%d|%d", time.Now().AddDate(-1, 0, 0).Year(), time.Now().Year())
	filter = map[string]string{
		"q": q,
		//"metadata": "",
	}

	action := "filter by " + q
	docs = searchDocuments(suite.T(), suite.userHttp, filter, 1, 10, "name", "ASC", 200)
	assert.Equal(suite.T(), 2, len(docs), action)
	assertDocInDocs(suite.T(), suite.docs["text-1"].Id, &docs, action)
	assertDocInDocs(suite.T(), suite.docs["text-2"].Id, &docs, action)

	action = "filter by " + q + ", sort desc"
	docs = searchDocuments(suite.T(), suite.userHttp, filter, 1, 10, "name", "ASC", 200)
	assert.Equal(suite.T(), 2, len(docs), action)
	assertDocInDocs(suite.T(), suite.docs["text-1"].Id, &docs, action)
	assertDocInDocs(suite.T(), suite.docs["text-2"].Id, &docs, action)

	// this year

	/*
			filter = map[string]string{
				"q": fmt.Sprintf("date:%d|%d-%d", time.Now().Year()-1, time.Now().Year(), time.Now().Month()-4),
				//"metadata": "",
			}
			action = serializeSearchQuery(filter)
			docs = searchDocuments(suite.T(), suite.userHttp, filter, 1, 10, "name", "ASC", 200)
			assert.Equal(suite.T(), 1, len(docs))
			assertDocInDocs(suite.T(), suite.docs["text-1"].Id, &docs, action)

		filter = map[string]string{
			"q": "author:doyle",
		}
		action = serializeSearchQuery(filter)
		docs = searchDocuments(suite.T(), suite.userHttp, filter, 1, 10, "name", "ASC", 200)
		assert.Equal(suite.T(), 1, len(docs))
		assertDocInDocs(suite.T(), suite.docs["text-2"].Id, &docs, action)

	*/

	filter = map[string]string{
		// typo
		"q": `lorem ipsom"`,
	}
	action = serializeSearchQuery(filter)
	docs = searchDocuments(suite.T(), suite.userHttp, filter, 1, 10, "name", "ASC", 200)
	assert.Equal(suite.T(), 2, len(docs))
	assertDocInDocs(suite.T(), suite.docs["text-1"].Id, &docs, action)
	assertDocInDocs(suite.T(), suite.docs["text-2"].Id, &docs, action)

	q = fmt.Sprintf("date:%d", time.Now().Year())
	filter = map[string]string{
		"q": q,
		//"metadata": "",
	}

	action = "filter by " + q
	docs = searchDocuments(suite.T(), suite.userHttp, filter, 1, 10, "name", "ASC", 200)
	assert.Equal(suite.T(), 1, len(docs), action)
	assertDocInDocs(suite.T(), suite.docs["text-1"].Id, &docs, action)

	q = fmt.Sprintf("date:%d", time.Now().AddDate(-1, 0, 0).Year())
	filter = map[string]string{
		"q": q,
		//"metadata": "",
	}

	action = "filter by " + q
	docs = searchDocuments(suite.T(), suite.userHttp, filter, 1, 10, "name", "ASC", 200)
	assert.Equal(suite.T(), 1, len(docs), action)
	assertDocInDocs(suite.T(), suite.docs["text-2"].Id, &docs, action)
}

func waitIndexingReady(t *testing.T, client *httpClient, timeoutSec int) {
	startTs := time.Now()
	for {
		if time.Now().Sub(startTs).Seconds() > float64(timeoutSec) {
			t.Errorf("timeout while waiting for indexing status")
			t.Skip()
			return
		}

		dto := &api.UserDocumentStatistics{}
		client.Get("/api/v1/documents/stats").Expect(t).Json(t, dto).e.Status(200).Done()

		if !dto.Indexing {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// filter is of type api.DocumentFilter
func searchDocuments(t *testing.T, client *httpClient, filter interface{}, page int, perPage int, sort string, order string, wantHttpStatus int) []*api.DocumentResponse {
	b, err := json.Marshal(filter)
	if err != nil {
		t.Errorf("marshal json filter: %v", err)
	}

	req := &httpRequest{client.Get("/api/v1/documents").
		// TODO: sort probably works incorrectly
		Sort(sort, order).Page(page, perPage).
		req.
		SetQuery("filter", string(b))}

	dto := &[]*api.DocumentResponse{}
	if wantHttpStatus == 200 {
		req.Expect(t).Json(t, dto).e.Status(200).Done()
		return *dto
	} else {
		req.Expect(t).e.Status(wantHttpStatus).Done()
		return nil
	}
}

func serializeSearchQuery(filter interface{}) string {
	b, _ := json.Marshal(filter)
	return string(b)
}

func assertDocInDocs(t *testing.T, docId string, docs *[]*api.DocumentResponse, msg string) {
	for _, v := range *docs {
		if docId == v.Id {
			return
		}
	}
	assert.Errorf(t, errors.New("expected document to exist in collection"), msg, docId, &docs)
	t.Fail()
}
