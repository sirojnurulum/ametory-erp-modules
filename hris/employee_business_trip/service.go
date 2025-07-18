package employee_business_trip

import (
	"net/http"
	"time"

	"github.com/AMETORY/ametory-erp-modules/context"
	"github.com/AMETORY/ametory-erp-modules/shared/models"
	"github.com/AMETORY/ametory-erp-modules/utils"
	"github.com/morkid/paginate"
	"gorm.io/gorm"
)

type EmployeeBusinessTripService struct {
	db  *gorm.DB
	ctx *context.ERPContext
}

func NewEmployeeBusinessTripService(ctx *context.ERPContext) *EmployeeBusinessTripService {
	return &EmployeeBusinessTripService{db: ctx.DB, ctx: ctx}
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.EmployeeBusinessTrip{},
		// &models.BusinessTripUsage{},
		// &models.BusinessTripRefund{},
	)
}

func (e *EmployeeBusinessTripService) CreateEmployeeBusinessTrip(employeeBusinessTrip *models.EmployeeBusinessTrip) error {
	return e.db.Create(employeeBusinessTrip).Error
}

func (e *EmployeeBusinessTripService) GetEmployeeBusinessTripByID(id string) (*models.EmployeeBusinessTrip, error) {
	var employeeBusinessTrip models.EmployeeBusinessTrip
	err := e.db.
		Preload("Employee", func(tx *gorm.DB) *gorm.DB {
			return tx.Preload("User").
				Preload("JobTitle").
				Preload("WorkLocation").
				Preload("WorkShift").
				Preload("Branch")
		}).
		Preload("Approver.User").
		Preload("ApprovalByAdmin").
		Preload("Company").
		Preload("TripParticipants", func(db *gorm.DB) *gorm.DB {
			return db.Preload("User").Preload("JobTitle")
		}).
		Preload("Approver.User").
		Where("id = ?", id).First(&employeeBusinessTrip).Error
	if err != nil {
		return nil, err
	}

	ticketFiles := []models.FileModel{}
	e.ctx.DB.Find(&ticketFiles, "ref_id = ? AND ref_type = ?", id, "employee_business_trip_transport_ticket")

	employeeBusinessTrip.TransportBookingFiles = ticketFiles

	hotelFiles := []models.FileModel{}
	e.ctx.DB.Find(&hotelFiles, "ref_id = ? AND ref_type = ?", id, "employee_business_trip_hotel_ticket")

	employeeBusinessTrip.HotelBookingFiles = hotelFiles

	return &employeeBusinessTrip, nil
}

func (e *EmployeeBusinessTripService) UpdateEmployeeBusinessTrip(employeeBusinessTrip *models.EmployeeBusinessTrip) error {
	return e.db.Save(employeeBusinessTrip).Error
}

func (e *EmployeeBusinessTripService) DeleteEmployeeBusinessTrip(id string) error {
	return e.db.Delete(&models.EmployeeBusinessTrip{}, "id = ?", id).Error
}

func (e *EmployeeBusinessTripService) FindAllByEmployeeID(request *http.Request, employeeID string) (paginate.Page, error) {
	pg := paginate.New()
	stmt := e.db.
		Preload("Company").
		Preload("Employee", func(tx *gorm.DB) *gorm.DB {
			return tx.Preload("User").
				Preload("JobTitle").
				Preload("WorkLocation").
				Preload("WorkShift").
				Preload("Branch")
		}).
		Preload("Approver.User").
		Preload("ApprovalByAdmin").
		Where("employee_id = ?", employeeID).
		Model(&models.EmployeeBusinessTrip{})
	if request.Header.Get("ID-Company") != "" {
		stmt = stmt.Where("company_id = ?", request.Header.Get("ID-Company"))
	}

	if request.URL.Query().Get("search") != "" {
		stmt = stmt.Where("reason LIKE ?", "%"+request.URL.Query().Get("search")+"%")
	}
	if request.URL.Query().Get("start_date") != "" && request.URL.Query().Get("end_date") != "" {
		stmt = stmt.Where("date >= ? AND date <= ?", request.URL.Query().Get("start_date"), request.URL.Query().Get("end_date"))
	} else if request.URL.Query().Get("start_date") != "" {
		stmt = stmt.Where("date = ?", request.URL.Query().Get("start_date"))
	}
	if request.URL.Query().Get("date") != "" {
		stmt = stmt.Where("DATE(date) = ?", request.URL.Query().Get("date"))
	}
	if request.URL.Query().Get("approver_id") != "" {
		stmt = stmt.Where("approver_id = ?", request.URL.Query().Get("approver_id"))
	}

	if request.URL.Query().Get("order") != "" {
		stmt = stmt.Order(request.URL.Query().Get("order"))
	} else {
		stmt = stmt.Order("date DESC")
	}
	utils.FixRequest(request)
	page := pg.With(stmt).Request(request).Response(&[]models.EmployeeBusinessTrip{})
	page.Page = page.Page + 1
	return page, nil
}
func (e *EmployeeBusinessTripService) FindAllEmployeeBusinessTrips(request *http.Request) (paginate.Page, error) {
	pg := paginate.New()
	stmt := e.db.
		Preload("Company").
		Preload("Employee", func(tx *gorm.DB) *gorm.DB {
			return tx.Preload("User").
				Preload("JobTitle").
				Preload("WorkLocation").
				Preload("WorkShift").
				Preload("Branch")
		}).
		Preload("Approver.User").
		Preload("ApprovalByAdmin").
		Preload("Reviewer").
		Model(&models.EmployeeBusinessTrip{})
	if request.Header.Get("ID-Company") != "" {
		stmt = stmt.Where("company_id = ?", request.Header.Get("ID-Company"))
	}

	if request.URL.Query().Get("search") != "" {
		stmt = stmt.Where("reason LIKE ?", "%"+request.URL.Query().Get("search")+"%")
	}
	if request.URL.Query().Get("start_date") != "" && request.URL.Query().Get("end_date") != "" {
		stmt = stmt.Where("date >= ? AND date <= ?", request.URL.Query().Get("start_date"), request.URL.Query().Get("end_date"))
	} else if request.URL.Query().Get("start_date") != "" {
		stmt = stmt.Where("date = ?", request.URL.Query().Get("start_date"))
	}
	if request.URL.Query().Get("date") != "" {
		stmt = stmt.Where("DATE(date) = ?", request.URL.Query().Get("date"))
	}
	if request.URL.Query().Get("approver_id") != "" {
		stmt = stmt.Where("approver_id = ?", request.URL.Query().Get("approver_id"))
	}

	if request.URL.Query().Get("order") != "" {
		stmt = stmt.Order(request.URL.Query().Get("order"))
	} else {
		stmt = stmt.Order("date DESC")
	}
	utils.FixRequest(request)
	page := pg.With(stmt).Request(request).Response(&[]models.EmployeeBusinessTrip{})
	page.Page = page.Page + 1
	return page, nil
}

func (e *EmployeeBusinessTripService) CountByEmployeeID(employeeID string, startDate *time.Time, endDate *time.Time) (map[string]int64, error) {
	var countREQUESTED, countAPPROVED, countREJECTED int64
	counts := make(map[string]int64)
	e.db.Model(&models.EmployeeBusinessTrip{}).
		Where("employee_id = ? AND status = ? AND date >= ? AND date <= ?", employeeID, "REQUESTED", startDate, endDate).
		Count(&countREQUESTED)
	e.db.Model(&models.EmployeeBusinessTrip{}).
		Where("employee_id = ? AND status = ? AND date >= ? AND date <= ?", employeeID, "APPROVED", startDate, endDate).
		Count(&countAPPROVED)
	e.db.Model(&models.EmployeeBusinessTrip{}).
		Where("employee_id = ? AND status = ? AND date >= ? AND date <= ?", employeeID, "REJECTED", startDate, endDate).
		Count(&countREJECTED)

	counts["REQUESTED"] = countREQUESTED
	counts["APPROVED"] = countAPPROVED
	counts["REJECTED"] = countREJECTED

	return counts, nil
}
