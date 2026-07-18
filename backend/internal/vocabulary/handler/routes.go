package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/memorix/memorix/internal/platform/httpx"
	"github.com/memorix/memorix/internal/vocabulary/domain"
	"github.com/memorix/memorix/internal/vocabulary/service"
)

// RegisterRoutes gắn route vocabulary vào group /api/v1.
func RegisterRoutes(g *gin.RouterGroup, h *Handler) {
	e := g.Group("/vocabulary")
	e.POST("/entries", h.create)
	e.GET("/entries", h.list)
	e.GET("/entries/:id", h.get)
	e.PATCH("/entries/:id", h.update)
	e.DELETE("/entries/:id", h.delete)
	e.GET("/curated-decks", h.listDecks)
	e.POST("/curated-decks/:id/enroll", h.enroll)
}

// ---- request/response DTOs ----

type meaningReq struct {
	PartOfSpeech string `json:"part_of_speech"`
	Definition   string `json:"definition"`
}
type pronReq struct {
	IPA      string `json:"ipa"`
	Dialect  string `json:"dialect"`
	AudioURL string `json:"audio_url"`
}
type entryReq struct {
	Term           string       `json:"term"`
	PartOfSpeech   string       `json:"part_of_speech"`
	Notes          string       `json:"notes"`
	Source         string       `json:"source"`
	Directions     []string     `json:"directions"`
	Meanings       []meaningReq `json:"meanings"`
	Examples       []string     `json:"examples"`
	Pronunciations []pronReq    `json:"pronunciations"`
	Synonyms       []string     `json:"synonyms"`
	Antonyms       []string     `json:"antonyms"`
}

func (r entryReq) toCreate() service.CreateEntryInput {
	in := service.CreateEntryInput{
		Term: r.Term, PartOfSpeech: r.PartOfSpeech, Notes: r.Notes, Source: r.Source,
		Directions: r.Directions, Examples: r.Examples, Synonyms: r.Synonyms, Antonyms: r.Antonyms,
	}
	for _, m := range r.Meanings {
		in.Meanings = append(in.Meanings, service.MeaningInput{PartOfSpeech: m.PartOfSpeech, Definition: m.Definition})
	}
	for _, p := range r.Pronunciations {
		in.Pronunciations = append(in.Pronunciations, service.PronunciationInput{IPA: p.IPA, Dialect: p.Dialect, AudioURL: p.AudioURL})
	}
	return in
}

func (r entryReq) toUpdate() service.UpdateEntryInput {
	c := r.toCreate()
	return service.UpdateEntryInput{
		Term: c.Term, PartOfSpeech: c.PartOfSpeech, Notes: c.Notes, Source: c.Source,
		Meanings: c.Meanings, Examples: c.Examples, Pronunciations: c.Pronunciations,
		Synonyms: c.Synonyms, Antonyms: c.Antonyms,
	}
}

func entryToJSON(e domain.Entry, status string) gin.H {
	meanings := make([]gin.H, 0, len(e.Meanings))
	for _, m := range e.Meanings {
		meanings = append(meanings, gin.H{"id": m.ID, "part_of_speech": m.PartOfSpeech, "definition": m.Definition, "position": m.Position})
	}
	examples := make([]gin.H, 0, len(e.Examples))
	for _, x := range e.Examples {
		examples = append(examples, gin.H{"id": x.ID, "text": x.Text, "position": x.Position})
	}
	prons := make([]gin.H, 0, len(e.Pronunciations))
	for _, p := range e.Pronunciations {
		prons = append(prons, gin.H{"id": p.ID, "ipa": p.IPA, "dialect": p.Dialect, "audio_url": p.AudioURL})
	}
	rels := make([]gin.H, 0, len(e.Relations))
	for _, s := range e.Relations {
		rels = append(rels, gin.H{"id": s.ID, "relation": string(s.Relation), "value": s.Value})
	}
	return gin.H{
		"id": e.ID, "term": e.Term, "part_of_speech": e.PartOfSpeech, "notes": e.Notes,
		"source": e.Source, "status": status, "created_at": e.CreatedAt, "updated_at": e.UpdatedAt,
		"meanings": meanings, "examples": examples, "pronunciations": prons, "synonyms_antonyms": rels,
	}
}

// ---- handlers ----

func (h *Handler) create(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	var req entryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErr(c, httpx.NewError(httpx.CodeValidation, "body không hợp lệ"))
		return
	}
	e, err := h.svc.Create(c.Request.Context(), owner, req.toCreate())
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": entryToJSON(e, "new")})
}

func (h *Handler) get(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	view, err := h.svc.Get(c.Request.Context(), owner, id)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entryToJSON(view.Entry, view.Status)})
}

func (h *Handler) update(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req entryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErr(c, httpx.NewError(httpx.CodeValidation, "body không hợp lệ"))
		return
	}
	e, err := h.svc.Update(c.Request.Context(), owner, id, req.toUpdate())
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entryToJSON(e, "")})
}

func (h *Handler) delete(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), owner, id); err != nil {
		mapErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) list(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	status := c.Query("status")
	if status != "" && !validStatuses[status] {
		writeErr(c, httpx.NewError(httpx.CodeValidation, "status không hợp lệ").WithField("status", "whitelist"))
		return
	}
	limit := 0
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	res, err := h.svc.List(c.Request.Context(), service.ListInput{
		OwnerID: owner, Status: status, Query: c.Query("q"), Cursor: c.Query("cursor"), Limit: limit,
	})
	if err != nil {
		mapErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(res.Items))
	for _, it := range res.Items {
		items = append(items, gin.H{
			"id": it.Entry.ID, "term": it.Entry.Term, "part_of_speech": it.Entry.PartOfSpeech,
			"status": it.Status, "created_at": it.Entry.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "page": res.Page})
}

func (h *Handler) listDecks(c *gin.Context) {
	if _, ok := h.owner(c); !ok {
		return
	}
	decks, err := h.svc.ListCuratedDecks(c.Request.Context())
	if err != nil {
		mapErr(c, err)
		return
	}
	out := make([]gin.H, 0, len(decks))
	for _, d := range decks {
		out = append(out, gin.H{"id": d.ID, "slug": d.Slug, "name": d.Name, "description": d.Description})
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *Handler) enroll(c *gin.Context) {
	owner, ok := h.owner(c)
	if !ok {
		return
	}
	deckID, ok := parseID(c, "id")
	if !ok {
		return
	}
	enrollmentID, err := h.svc.Enroll(c.Request.Context(), owner, deckID)
	if err != nil {
		mapErr(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"enrollment_id": enrollmentID, "status": "pending"}})
}
