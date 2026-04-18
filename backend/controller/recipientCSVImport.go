package controller

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/vo"
)

// ImportCSV imports recipients from a CSV file upload.
// POST /api/v1/recipient/import-csv
// Expects multipart form with "file" field (CSV) and optional "companyID" field.
func (r *Recipient) ImportCSV(g *gin.Context) {
	session, _, ok := r.handleSession(g)
	if !ok {
		return
	}

	// Parse the uploaded file
	file, _, err := g.Request.FormFile("file")
	if err != nil {
		r.Response.BadRequestMessage(g, "missing or invalid CSV file")
		return
	}
	defer file.Close()

	// Parse optional companyID
	var companyID *uuid.UUID
	if cidStr := g.PostForm("companyID"); cidStr != "" {
		if cid, err := uuid.Parse(cidStr); err == nil {
			companyID = &cid
		}
	}

	// Parse CSV
	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	reader.LazyQuotes = true

	// Read header
	header, err := reader.Read()
	if err != nil {
		r.Response.BadRequestMessage(g, "failed to read CSV header")
		return
	}

	// Build header map (lowercase)
	headerMap := make(map[string]int)
	for i, h := range header {
		headerMap[strings.TrimSpace(strings.ToLower(h))] = i
	}

	// Require email column
	emailIdx, hasEmail := headerMap["email"]
	if !hasEmail {
		r.Response.BadRequestMessage(g, "CSV must have an 'email' column")
		return
	}

	// Optional column indices
	getIdx := func(names ...string) int {
		for _, n := range names {
			if idx, ok := headerMap[n]; ok {
				return idx
			}
		}
		return -1
	}

	phoneIdx := getIdx("phone")
	extraIdx := getIdx("extraidentifier", "extra_identifier", "extra identifier")
	firstNameIdx := getIdx("firstname", "first_name", "first name")
	lastNameIdx := getIdx("lastname", "last_name", "last name")
	positionIdx := getIdx("position")
	departmentIdx := getIdx("department")
	cityIdx := getIdx("city")
	countryIdx := getIdx("country")
	miscIdx := getIdx("misc")

	getField := func(row []string, idx int) string {
		if idx < 0 || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	var recipients []*model.Recipient
	lineNum := 1 // header is line 1
	skipped := 0

	for {
		lineNum++
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			skipped++
			continue
		}

		email := ""
		if emailIdx < len(row) {
			email = strings.TrimSpace(row[emailIdx])
		}
		if email == "" {
			skipped++
			continue
		}

		emailVO, err := vo.NewEmail(email)
		if err != nil {
			skipped++
			continue
		}

		rec := &model.Recipient{
			Email: nullable.NewNullableWithValue(*emailVO),
		}

		if v := getField(row, phoneIdx); v != "" {
			rec.Phone = nullable.NewNullableWithValue(*vo.NewOptionalString127Must(v))
		}
		if v := getField(row, extraIdx); v != "" {
			rec.ExtraIdentifier = nullable.NewNullableWithValue(*vo.NewOptionalString127Must(v))
		}
		if v := getField(row, firstNameIdx); v != "" {
			rec.FirstName = nullable.NewNullableWithValue(*vo.NewOptionalString127Must(v))
		}
		if v := getField(row, lastNameIdx); v != "" {
			rec.LastName = nullable.NewNullableWithValue(*vo.NewOptionalString127Must(v))
		}
		if v := getField(row, positionIdx); v != "" {
			rec.Position = nullable.NewNullableWithValue(*vo.NewOptionalString127Must(v))
		}
		if v := getField(row, departmentIdx); v != "" {
			rec.Department = nullable.NewNullableWithValue(*vo.NewOptionalString127Must(v))
		}
		if v := getField(row, cityIdx); v != "" {
			rec.City = nullable.NewNullableWithValue(*vo.NewOptionalString127Must(v))
		}
		if v := getField(row, countryIdx); v != "" {
			rec.Country = nullable.NewNullableWithValue(*vo.NewOptionalString127Must(v))
		}
		if v := getField(row, miscIdx); v != "" {
			rec.Misc = nullable.NewNullableWithValue(*vo.NewOptionalString127Must(v))
		}

		recipients = append(recipients, rec)
	}

	if len(recipients) == 0 {
		r.Response.BadRequestMessage(g, fmt.Sprintf("no valid recipients found in CSV (%d rows skipped)", skipped))
		return
	}

	result, err := r.RecipientService.Import(
		g,
		session,
		recipients,
		true, // ignoreOverwriteEmptyFields
		companyID,
	)
	if ok := r.handleErrors(g, err); !ok {
		return
	}

	// Attach skipped count to result
	response := map[string]interface{}{
		"result":  result,
		"skipped": skipped,
		"parsed":  len(recipients),
	}

	r.Response.OK(g, response)
}
