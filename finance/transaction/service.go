package transaction

import (
	"fmt"
	"net/http"
	"time"

	"github.com/AMETORY/ametory-erp-modules/context"
	"github.com/AMETORY/ametory-erp-modules/finance/account"
	"github.com/AMETORY/ametory-erp-modules/shared/models"
	"github.com/AMETORY/ametory-erp-modules/utils"
	"github.com/google/uuid"
	"github.com/morkid/paginate"
	"gorm.io/gorm"
)

type TransactionService struct {
	db             *gorm.DB
	ctx            *context.ERPContext
	accountService *account.AccountService
}

// NewTransactionService returns a new instance of TransactionService.
//
// The service is created by providing a GORM database instance, an ERP context,
// and an AccountService. The ERP context is used for authentication and
// authorization purposes, while the database instance is used for CRUD (Create,
// Read, Update, Delete) operations. The AccountService is used to fetch related
// account data for transactions.

func NewTransactionService(db *gorm.DB, ctx *context.ERPContext, accountService *account.AccountService) *TransactionService {
	return &TransactionService{db: db, ctx: ctx, accountService: accountService}
}

// Migrate runs the database migration for the transaction module. It creates the
// transactions table with the required columns and indexes.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&models.TransactionModel{})
}
func (s *TransactionService) SetDB(db *gorm.DB) {
	s.db = db
}

// CreateTransaction creates a new transaction in the database. If the
// transaction's AccountID is set, the transaction is associated with the
// specified account. If the transaction's SourceID is set, a transfer
// transaction is created.
func (s *TransactionService) CreateTransaction(transaction *models.TransactionModel, amount float64) error {
	code := utils.RandString(10, false)
	if transaction.AccountID != nil {
		if transaction.ID == "" {
			transaction.ID = uuid.New().String()
		}
		transaction.Code = code
		transaction.Amount = amount
		account, err := s.accountService.GetAccountByID(*transaction.AccountID)
		if err != nil {
			return err
		}
		s.UpdateCreditDebit(transaction, account.Type)

		if err := s.db.Create(transaction).Error; err != nil {
			return err
		}
	} else {
		var transSourceID, transDestID string = uuid.New().String(), uuid.New().String()
		if transaction.SourceID != nil {
			transaction.ID = transSourceID
			transaction.Code = code
			transaction.AccountID = transaction.SourceID
			transaction.Amount = amount
			if transaction.TransactionRefID == nil {
				transaction.TransactionRefID = &transDestID
			}
			if transaction.TransactionRefType == "" {
				transaction.TransactionRefType = "transaction"
			}
			account, err := s.accountService.GetAccountByID(*transaction.SourceID)
			if err != nil {
				return err
			}
			s.UpdateCreditDebit(transaction, account.Type)
			if transaction.IsTransfer {
				transaction.Credit = amount
				transaction.Debit = 0
			}
			if account.Type == models.EQUITY {
				if transaction.Amount < 0 {
					transaction.Credit = 0
					transaction.Debit = -transaction.Amount
				}
			}

			if err := s.db.Create(transaction).Error; err != nil {
				return err
			}
		}
		if transaction.DestinationID != nil {
			transaction.ID = transDestID
			transaction.Code = code
			transaction.AccountID = transaction.DestinationID
			transaction.Amount = amount
			transaction.TransactionRefID = &transSourceID
			if transaction.TransactionRefType == "" {
				transaction.TransactionRefType = "transaction"
			}
			account, err := s.accountService.GetAccountByID(*transaction.DestinationID)
			if err != nil {
				return err
			}
			s.UpdateCreditDebit(transaction, account.Type)
			if transaction.IsTransfer {
				transaction.Debit = amount
				transaction.Credit = 0
				transaction.IsTransfer = false
			}
			if account.Type == models.ASSET {
				transaction.IsIncome = false
				transaction.IsExpense = false
				if transaction.Amount < 0 {
					transaction.Debit = 0
					transaction.Credit = -transaction.Amount
				}
			}
			if err := s.db.Create(transaction).Error; err != nil {
				return err
			}

		}
	}

	return nil
}

// UpdateTransaction updates a transaction by its ID. It takes a string ID and a pointer
// to a TransactionModel as its arguments. The TransactionModel instance contains the
// updated values for the transaction.
//
// The method returns an error if the update operation fails. If the update is
// successful, the error is nil.
//
// The method is run inside a transaction. If the transaction has a counter-part
// transaction with the same code, the counter-part transaction is updated as well.
func (s *TransactionService) UpdateTransaction(id string, transaction *models.TransactionModel) error {
	// return s.db.Where("id = ?", id).Updates(transaction).Error
	return s.db.Transaction(func(tx *gorm.DB) error {
		if transaction.Debit > 0 {
			transaction.Debit = transaction.Amount
		}
		if transaction.Credit > 0 {
			transaction.Credit = transaction.Amount
		}
		err := tx.Model(&models.TransactionModel{}).Where("id = ?", id).Updates(transaction).Error
		if err != nil {
			return err
		}
		var trans2 models.TransactionModel
		err = tx.Where("code = ? and id != ?", transaction.Code, transaction.ID).First(&trans2).Error
		if err == nil {
			var credit, debit float64 = transaction.Debit, transaction.Credit
			if credit > 0 {
				credit = transaction.Amount
			}
			if debit > 0 {
				debit = transaction.Amount
			}
			err = tx.Model(&models.TransactionModel{}).Where("id = ?", trans2.ID).Updates(map[string]any{
				"credit":      credit,
				"debit":       debit,
				"description": transaction.Description,
				"date":        transaction.Date,
				"amount":      transaction.Amount,
			}).Error
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// DeleteTransaction deletes a transaction by its ID.
//
// It returns an error if the deletion operation fails. Before deleting the
// transaction, it retrieves the transaction data to get the transaction code.
// After deleting the transaction, it deletes the counter-part transaction with
// the same code.
func (s *TransactionService) DeleteTransaction(id string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var data models.TransactionModel
		err := tx.Where("id = ?", id).First(&data).Error
		if err != nil {
			return err
		}
		err = tx.Where("id = ?", id).Delete(&models.TransactionModel{}).Error
		if err != nil {
			return err
		}
		err = tx.Where("code = ?", data.Code).Delete(&models.TransactionModel{}).Error
		if err != nil {
			return err
		}
		return nil
	})
}

// GetTransactionById retrieves a transaction by its ID.
//
// It takes the ID of the transaction as an argument and returns a pointer to a
// TransactionModel and an error. The function uses GORM to retrieve the
// transaction data from the transactions table. If the operation fails, an error
// is returned.
func (s *TransactionService) GetTransactionById(id string) (*models.TransactionModel, error) {
	var transaction models.TransactionModel
	err := s.db.Preload("Account").Select("transactions.*, accounts.name as account_name").Joins("LEFT JOIN accounts ON accounts.id = transactions.account_id").
		First(&transaction, "transactions.id = ?", id).Error
	return &transaction, err
}

// GetTransactionByCode retrieves a transaction by its code.
//
// It takes a code as input and returns a slice of TransactionModel and an error.
// The function uses GORM to retrieve the transaction data from the transactions
// table. If the operation fails, an error is returned. Otherwise, the error is
// nil.
func (s *TransactionService) GetTransactionByCode(code string) ([]models.TransactionModel, error) {
	var transaction []models.TransactionModel
	err := s.db.Preload("Account").Select("transactions.*, accounts.name as account_name").Joins("LEFT JOIN accounts ON accounts.id = transactions.account_id").
		First(&transaction, "transactions.code = ?", code).Error
	return transaction, err
}

// GetTransactionByDate retrieves a paginated list of transactions that occurred
// between the specified 'from' and 'to' dates. The function filters transactions
// by company ID if it is provided in the request header. It returns a paginated
// page of TransactionModel and an error if the operation fails.

func (s *TransactionService) GetTransactionByDate(from, to time.Time, request http.Request) (paginate.Page, error) {
	pg := paginate.New()
	stmt := s.db.Where("date BETWEEN ? AND ?", from, to)
	if request.Header.Get("ID-Company") != "" {
		stmt = stmt.Where("company_id = ?", request.Header.Get("ID-Company"))
	}
	utils.FixRequest(&request)
	page := pg.With(stmt).Request(request).Response(new([]models.TransactionModel))
	page.Page = page.Page + 1
	return page, nil
}

// GetByDateAndCompanyId retrieves a paginated list of transactions that occurred
// between the specified 'from' and 'to' dates for a given company ID. The
// function filters transactions by date and company ID, and returns a slice of
// TransactionModel and an error if the operation fails. The function uses
// pagination to manage the result set.
func (s *TransactionService) GetByDateAndCompanyId(from, to time.Time, companyId string, page, limit int) ([]models.TransactionModel, error) {
	var transactions []models.TransactionModel
	err := s.db.Where("date BETWEEN ? AND ? AND company_id = ?", from, to, companyId).
		Offset((page - 1) * limit).Limit(limit).Find(&transactions).Error
	return transactions, err
}

// GetTransactionsByAccountID retrieves a paginated list of transactions for a given account ID.
//
// It takes an account ID, optional start and end dates, and an optional company ID as input.
// The function filters transactions by account ID, company ID, and date range, and returns a
// paginated page of TransactionModel and an error if the operation fails.
// The function uses pagination to manage the result set.
// Each item in the result set has its balance calculated using the getBalance function.
func (s *TransactionService) GetTransactionsByAccountID(accountID string, startDate *time.Time, endDate *time.Time, companyID *string, request http.Request) (paginate.Page, error) {
	pg := paginate.New()
	stmt := s.db.Preload("Account").Select("transactions.*, accounts.name as account_name").Joins("LEFT JOIN accounts ON accounts.id = transactions.account_id")
	if companyID != nil {
		stmt = stmt.Where("transactions.company_id = ?", *companyID)
	}
	if startDate != nil {
		stmt = stmt.Where("transactions.date >= ?", *startDate)
	}
	if endDate != nil {
		stmt = stmt.Where("transactions.date < ?", *endDate)
	}
	stmt = stmt.Where("transactions.account_id = ?", accountID)
	stmt = stmt.Model(&models.TransactionModel{})
	utils.FixRequest(&request)
	page := pg.With(stmt).Request(request).Response(&[]models.TransactionModel{})
	page.Page = page.Page + 1
	items := page.Items.(*[]models.TransactionModel)
	newItems := make([]models.TransactionModel, 0)
	for _, item := range *items {
		if item.TransactionRefID != nil {
			var transRef models.TransactionModel
			err := s.db.Preload("Account").Where("id = ?", item.TransactionRefID).First(&transRef).Error
			if err == nil {
				item.TransactionRef = &transRef
			}
		}
		item.Balance = s.getBalance(item)
		newItems = append(newItems, item)
	}
	page.Items = &newItems
	return page, nil
}

// GetTransactions retrieves a paginated list of transactions from the database.
//
// It takes an HTTP request and a search query string as input. The method uses
// GORM to query the database for transactions, applying the search query to the
// account name, code, description, and various other fields. If the request contains
// a company ID header, the method filters the result by the company ID.
// The function utilizes pagination to manage the result set and includes any
// necessary request modifications using the utils.FixRequest utility.
// The function returns a paginated page of TransactionModel and an error if the
// operation fails.
func (s *TransactionService) GetTransactions(request http.Request, search string) (paginate.Page, error) {
	pg := paginate.New()
	stmt := s.db.Preload("Account").Select("transactions.*, accounts.name as account_name").Joins("LEFT JOIN accounts ON accounts.id = transactions.account_id")
	if request.Header.Get("ID-Company") != "" {
		stmt = stmt.Where("transactions.company_id = ?", request.Header.Get("ID-Company"))
	}
	if search != "" {
		stmt = stmt.Where("accounts.name ILIKE ? OR accounts.code ILIKE ? OR transactions.code ILIKE ? OR transactions.description ILIKE ?",
			"%"+search+"%",
			"%"+search+"%",
			"%"+search+"%",
			"%"+search+"%",
		)
	}

	switch request.URL.Query().Get("type") {
	case "INCOME", "REVENUE":
		stmt = stmt.Where("transactions.is_income = ?", true)
	case "EXPENSE":
		stmt = stmt.Where("transactions.is_expense = ?", true)
	case "EQUITY":
		stmt = stmt.Where("transactions.is_equity = ?", true)
	case "TRANSFER":
		stmt = stmt.Where("transactions.is_transfer = ?", true)
	case "LIABILITY", "PAYABLE":
		stmt = stmt.Where("transactions.is_account_payable = ?", true)
	case "RECEIVABLE":
		stmt = stmt.Where("transactions.is_account_receivable = ?", true)
	}

	if request.URL.Query().Get("account_id") != "" {
		stmt = stmt.Where("transactions.account_id = ?", request.URL.Query().Get("account_id"))
	}

	if request.URL.Query().Get("start_date") != "" && request.URL.Query().Get("end_date") != "" {
		stmt = stmt.Where("transactions.date between ? and ?", request.URL.Query().Get("start_date"), request.URL.Query().Get("end_date"))
	} else if request.URL.Query().Get("start_date") != "" {
		stmt = stmt.Where("transactions.date >= ?", request.URL.Query().Get("start_date"))
	} else if request.URL.Query().Get("end_date") != "" {
		stmt = stmt.Where("transactions.date <= ?", request.URL.Query().Get("end_date"))
	}

	stmt = stmt.Model(&models.TransactionModel{})
	utils.FixRequest(&request)
	page := pg.With(stmt).Request(request).Response(&[]models.TransactionModel{})
	page.Page = page.Page + 1
	items := page.Items.(*[]models.TransactionModel)
	newItems := make([]models.TransactionModel, 0)
	for _, item := range *items {
		if item.TransactionRefID != nil {
			if item.TransactionRefType == "journal" {
				var journalRef models.JournalModel
				err := s.db.Where("id = ?", item.TransactionRefID).First(&journalRef).Error
				if err == nil {
					item.JournalRef = &journalRef
				}
			}
			if item.TransactionRefType == "transaction" {
				var transRef models.TransactionModel
				err := s.db.Preload("Account").Where("id = ?", item.TransactionRefID).First(&transRef).Error
				if err == nil {
					item.TransactionRef = &transRef
				}
			}
			if item.TransactionRefType == "sales" {
				var salesRef models.SalesModel
				err := s.db.Where("id = ?", item.TransactionRefID).First(&salesRef).Error
				if err == nil {
					item.SalesRef = &salesRef
				}
			}
			if item.TransactionRefType == "purchase" {
				var purchaseRef models.PurchaseOrderModel
				err := s.db.Where("id = ?", item.TransactionRefID).First(&purchaseRef).Error
				if err == nil {
					item.PurchaseRef = &purchaseRef
				}
			}
		}
		item.Balance = s.getBalance(item)
		newItems = append(newItems, item)
	}
	page.Items = &newItems
	return page, nil
}

// UpdateCreditDebit updates the debit and credit values of a transaction based on the given account type.
// It also sets the appropriate flags for expense, income, and equity transactions.
// The function returns the updated transaction, or an error if the account type is unrecognized.
func (s *TransactionService) UpdateCreditDebit(transaction *models.TransactionModel, accountType models.AccountType) (*models.TransactionModel, error) {
	// transaction.IsExpense = false
	// transaction.IsIncome = false

	if accountType == models.EXPENSE {
		transaction.IsExpense = true
	}
	if accountType == models.REVENUE || accountType == models.INCOME {
		transaction.IsIncome = true
	}
	if accountType == models.EQUITY {
		transaction.IsEquity = true
	}
	if accountType == models.LIABILITY || accountType == models.PAYABLE {
		transaction.IsAccountPayable = true
	}
	if accountType == models.RECEIVABLE {
		transaction.IsAccountReceivable = true
	}
	switch accountType {
	case models.EXPENSE, models.COST, models.CONTRA_LIABILITY, models.CONTRA_EQUITY, models.CONTRA_REVENUE, models.RECEIVABLE:
		transaction.Debit = transaction.Amount
		transaction.Credit = 0
	case models.LIABILITY, models.EQUITY, models.REVENUE, models.INCOME, models.CONTRA_ASSET, models.CONTRA_EXPENSE:
		transaction.Credit = transaction.Amount
		transaction.Debit = 0

	case models.ASSET:
		transaction.Debit = transaction.Amount
		transaction.Credit = 0
		if transaction.IsIncome {
			transaction.Debit = transaction.Amount
			transaction.Credit = 0
		}
		if transaction.IsEquity {
			transaction.Debit = transaction.Amount
			transaction.Credit = 0
			transaction.IsEquity = false
		}
		if transaction.IsExpense {
			transaction.Credit = transaction.Amount
			transaction.Debit = 0
		}

	default:
		return transaction, fmt.Errorf("unhandled account type: %s", accountType)
	}

	fmt.Printf("account type: %v, is income: %v, is expense: %v, is equity: %v\n", accountType, transaction.IsIncome, transaction.IsExpense, transaction.IsEquity)

	return transaction, nil
}

// getBalance takes a transaction and returns the balance of the transaction.
// It uses the type of the account to determine whether the balance is
// calculated as debit - credit or credit - debit.
func (s *TransactionService) getBalance(transaction models.TransactionModel) float64 {
	switch transaction.Account.Type {
	case models.EXPENSE, models.COST, models.CONTRA_LIABILITY, models.CONTRA_EQUITY, models.CONTRA_REVENUE, models.RECEIVABLE:
		return transaction.Debit - transaction.Credit
	case models.LIABILITY, models.EQUITY, models.REVENUE, models.INCOME, models.CONTRA_ASSET, models.CONTRA_EXPENSE:
		return transaction.Credit - transaction.Debit
	case models.ASSET:
		return transaction.Debit - transaction.Credit
	}
	return 0
}
