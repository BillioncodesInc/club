package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	"github.com/phishingclub/phishingclub/database"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/vo"
	"gorm.io/gorm"
)

var openRedirectAllowedColumns = assignTableToColumns(database.OPEN_REDIRECT_TABLE, []string{
	"created_at",
	"updated_at",
	"name",
	"platform",
	"is_verified",
	"use_with_proxy",
})

// OpenRedirectOption is for eager loading
type OpenRedirectOption struct {
	Fields      []string
	*vo.QueryArgs
	WithCompany bool
	WithProxy   bool
}

// OpenRedirect is an open redirect repository
type OpenRedirect struct {
	DB *gorm.DB
}

// load preloads the table relations
func (m *OpenRedirect) load(
	options *OpenRedirectOption,
	db *gorm.DB,
) *gorm.DB {
	if options.WithCompany {
		db = db.Joins("Company")
	}
	if options.WithProxy {
		db = db.Joins("Proxy")
	}
	return db
}

// Insert inserts an open redirect
func (m *OpenRedirect) Insert(
	ctx context.Context,
	redirect *model.OpenRedirect,
) (*uuid.UUID, error) {
	id := uuid.New()
	row := redirect.ToDBMap()
	row["id"] = id
	AddTimestamps(row)
	res := m.DB.
		Model(&database.OpenRedirect{}).
		Create(row)
	if res.Error != nil {
		return nil, errs.Wrap(res.Error)
	}
	return &id, nil
}

// GetAll gets all open redirects with pagination
func (m *OpenRedirect) GetAll(
	ctx context.Context,
	companyID *uuid.UUID,
	options *OpenRedirectOption,
) (*model.Result[model.OpenRedirect], error) {
	result := model.NewEmptyResult[model.OpenRedirect]()
	var rows []database.OpenRedirect

	db := m.load(options, m.DB)
	db = withCompanyIncludingNullContext(db, companyID, database.OPEN_REDIRECT_TABLE)

	db, err := useQuery(db, database.OPEN_REDIRECT_TABLE, options.QueryArgs, openRedirectAllowedColumns...)
	if err != nil {
		return result, errs.Wrap(err)
	}

	if options.Fields != nil {
		fields := assignTableToColumns(database.OPEN_REDIRECT_TABLE, options.Fields)
		db = db.Select(strings.Join(fields, ","))
	}

	dbRes := db.Find(&rows)
	if dbRes.Error != nil {
		return result, dbRes.Error
	}

	hasNextPage, err := useHasNextPage(db, database.OPEN_REDIRECT_TABLE, options.QueryArgs, openRedirectAllowedColumns...)
	if err != nil {
		return result, errs.Wrap(err)
	}
	result.HasNextPage = hasNextPage

	for _, row := range rows {
		r, err := ToOpenRedirect(&row)
		if err != nil {
			return result, errs.Wrap(err)
		}
		result.Rows = append(result.Rows, r)
	}
	return result, nil
}

// GetByID gets an open redirect by ID
func (m *OpenRedirect) GetByID(
	ctx context.Context,
	id *uuid.UUID,
	options *OpenRedirectOption,
) (*model.OpenRedirect, error) {
	dbRedirect := database.OpenRedirect{}
	db := m.load(options, m.DB)
	result := db.
		Where(TableColumnID(database.OPEN_REDIRECT_TABLE)+" = ?", id).
		First(&dbRedirect)
	if result.Error != nil {
		return nil, result.Error
	}
	return ToOpenRedirect(&dbRedirect)
}

// UpdateByID updates an open redirect by id
func (m *OpenRedirect) UpdateByID(
	ctx context.Context,
	id *uuid.UUID,
	redirect *model.OpenRedirect,
) error {
	row := redirect.ToDBMap()
	AddUpdatedAt(row)
	result := m.DB.
		Model(&database.OpenRedirect{}).
		Where("id = ?", id).
		Updates(row)
	if result.Error != nil {
		return errs.Wrap(result.Error)
	}
	return nil
}

// DeleteByID deletes an open redirect by id
func (m *OpenRedirect) DeleteByID(
	ctx context.Context,
	id *uuid.UUID,
) error {
	result := m.DB.
		Where("id = ?", id).
		Delete(&database.OpenRedirect{})
	if result.Error != nil {
		return errs.Wrap(result.Error)
	}
	return nil
}

// GetAllByPlatform gets all open redirects for a specific platform
func (m *OpenRedirect) GetAllByPlatform(
	ctx context.Context,
	platform string,
	companyID *uuid.UUID,
) ([]model.OpenRedirect, error) {
	var rows []database.OpenRedirect
	db := m.DB
	db = withCompanyIncludingNullContext(db, companyID, database.OPEN_REDIRECT_TABLE)
	result := db.
		Where(fmt.Sprintf("%s = ?", TableColumn(database.OPEN_REDIRECT_TABLE, "platform")), platform).
		Find(&rows)
	if result.Error != nil {
		return nil, errs.Wrap(result.Error)
	}
	redirects := make([]model.OpenRedirect, len(rows))
	for i, row := range rows {
		r, err := ToOpenRedirect(&row)
		if err != nil {
			return nil, err
		}
		redirects[i] = *r
	}
	return redirects, nil
}

// GetVerified gets all verified open redirects
func (m *OpenRedirect) GetVerified(
	ctx context.Context,
	companyID *uuid.UUID,
) ([]model.OpenRedirect, error) {
	var rows []database.OpenRedirect
	db := m.DB
	db = withCompanyIncludingNullContext(db, companyID, database.OPEN_REDIRECT_TABLE)
	result := db.
		Where(fmt.Sprintf("%s = ?", TableColumn(database.OPEN_REDIRECT_TABLE, "is_verified")), true).
		Find(&rows)
	if result.Error != nil {
		return nil, errs.Wrap(result.Error)
	}
	redirects := make([]model.OpenRedirect, len(rows))
	for i, row := range rows {
		r, err := ToOpenRedirect(&row)
		if err != nil {
			return nil, err
		}
		redirects[i] = *r
	}
	return redirects, nil
}

// ToOpenRedirect converts a database.OpenRedirect to a model.OpenRedirect
func ToOpenRedirect(row *database.OpenRedirect) (*model.OpenRedirect, error) {
	id := nullable.NewNullableWithValue(*row.ID)
	companyID := nullable.NewNullNullable[uuid.UUID]()
	if row.CompanyID != nil {
		companyID.Set(*row.CompanyID)
	}
	proxyID := nullable.NewNullNullable[uuid.UUID]()
	if row.ProxyID != nil {
		proxyID.Set(*row.ProxyID)
	}
	name := nullable.NewNullableWithValue(*vo.NewString64Must(row.Name))
	baseURL := nullable.NewNullableWithValue(*vo.NewString1024Must(row.BaseURL))
	platform := nullable.NewNullableWithValue(*vo.NewString64Must(row.Platform))
	paramName := nullable.NewNullableWithValue(*vo.NewString64Must(row.ParamName))

	notes, err := vo.NewOptionalString1024(row.Notes)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	notesNullable := nullable.NewNullableWithValue(*notes)

	return &model.OpenRedirect{
		ID:             id,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		CompanyID:      companyID,
		BaseURL:        baseURL,
		Name:           name,
		Platform:       platform,
		ParamName:      paramName,
		IsVerified:     row.IsVerified,
		LastTestedAt:   row.LastTestedAt,
		LastStatusCode: &row.LastStatusCode,
		UseWithProxy:   &row.UseWithProxy,
		ProxyID:        proxyID,
		Notes:          notesNullable,
	}, nil
}
