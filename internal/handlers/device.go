package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/CLDWare/schoolbox-backend/config"
	models "github.com/CLDWare/schoolbox-backend/pkg/db"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"
	"github.com/MonkyMars/gecho"
	"gorm.io/gorm"
)

// UserHandler handles requests about users
type DeviceHandler struct {
	quitCh           chan os.Signal
	config           *config.Config
	db               *gorm.DB
	websocketHandler *WebsocketHandler
}

// NewDeviceHandler creates a new DeviceHandler
func NewDeviceHandler(quitCh chan os.Signal, cfg *config.Config, db *gorm.DB, websocketHandler *WebsocketHandler) *DeviceHandler {
	return &DeviceHandler{
		quitCh:           quitCh,
		config:           cfg,
		db:               db,
		websocketHandler: websocketHandler,
	}
}

type DeviceInfo struct {
	ID               uint       `json:"id"`
	LatestLogin      *time.Time `json:"latest_login"`
	LastSeen         *time.Time `json:"last_seen"`
	Room             *string    `json:"room"`
	LeaseStart       time.Time  `json:"lease_start"`
	ActiveSessionID  *uint      `json:"active_session_id"`
	RegistrationDate time.Time  `json:"registration_date"`
}

func toDeviceInfo(device models.Device) DeviceInfo {
	return DeviceInfo{
		ID:               device.ID,
		LatestLogin:      device.LatestLogin,
		LastSeen:         device.LastSeen,
		Room:             device.Room,
		LeaseStart:       device.LeaseStart,
		ActiveSessionID:  device.ActiveSessionID,
		RegistrationDate: device.RegistrationDate,
	}
}

// GetDevice
//
// @Summary		Get all devices
// @Description	Get DeviceInfo about all devices
// @Tags			device requiresAuth requiresAdmin
// @Accept			json
// @Produce		json
// @Param			limit	query		int	false	"Amount of devices to return" default(20) maximum(20)
// @Param			offset	query		int	false	"How much devices to skip before starting to return devices" default(0) minimum(0)
// @Param			leased	query		bool	false	"Only return devices with this lease status"
// @Success		200	{object}	apiResponses.BaseResponse{data=[]DeviceInfo}
// @Failure		401	{object}	apiResponses.UnauthorizedError
// @Failure		403	{object}	apiResponses.ForbiddenError
// @Failure		500	{object}	apiResponses.InternalServerError
// @Router			/device [get]
func (h *DeviceHandler) GetDevice(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodGet); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}

	query := r.URL.Query()
	dbQuery := h.db.Model(&models.Device{})

	// return count filters
	if limitStr := query.Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			gecho.BadRequest(w).WithMessage(err.Error()).Send()
			return
		}
		if limit > 20 {
			limit = 20
		}
		dbQuery = dbQuery.Limit(limit)
	} else {
		dbQuery = dbQuery.Limit(20)
	}
	if offsetStr := query.Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			gecho.BadRequest(w).WithMessage(err.Error()).Send()
			return
		}
		dbQuery = dbQuery.Offset(offset)
	}
	// filters
	if leasedStr := query.Get("leased"); leasedStr != "" {
		leased, err := strconv.ParseBool(leasedStr)
		if err != nil {
			gecho.BadRequest(w).WithMessage(err.Error()).Send()
			return
		}
		if leased {
			dbQuery = dbQuery.Where("active_session_id IS NOT NULL")
		} else {
			dbQuery = dbQuery.Where("active_session_id IS NULL")
		}
	}

	var devices []models.Device
	err := dbQuery.Find(&devices).Error
	if err != nil {
		gecho.InternalServerError(w).Send()
		logger.Err(err.Error())
		return
	}

	deviceInfoArray := []DeviceInfo{}
	for _, device := range devices {
		deviceInfoArray = append(deviceInfoArray, toDeviceInfo(device))
	}

	gecho.Success(w).WithData(deviceInfoArray).Send()
}

// GetDeviceById
//
// @Summary		Get device by id
// @Description	Get info about a device by using its id or room
// @Tags			device requiresAuth requiresAdmin
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Device ID or Room"
// @Param			type	query		string	false	"Specify identifier type" Enums("id","room") default("id")
// @Success		200 {object}	apiResponses.BaseResponse{data=DeviceInfo}
// @Failure		401	{object}	apiResponses.UnauthorizedError
// @Failure		403	{object}	apiResponses.ForbiddenError
// @Failure		404	{object}	apiResponses.NotFoundError
// @Failure		500	{object}	apiResponses.InternalServerError
// @Router			/device/{id} [get]
func (h *DeviceHandler) GetDeviceById(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodGet); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}

	query := r.URL.Query()
	dbQuery := h.db.Model(&models.Device{})

	idStr := r.PathValue("id")
	idType := query.Get("type")
	if idType == "" {
		idType = "id"
	}

	switch idType {
	case "id":
		userID, err := strconv.ParseUint(idStr, 10, 0)
		if err != nil {
			gecho.BadRequest(w).WithMessage("Invalid device ID, expected positive integer").Send()
			return
		}
		dbQuery = dbQuery.Where("id = ?", userID)
	case "room":
		dbQuery = dbQuery.Where("room = ?", idStr)
	default:
		gecho.BadRequest(w).WithMessage(fmt.Sprintf("Invalid identifier type '%s'", idType)).Send()
		return
	}

	var device models.Device
	result := dbQuery.First(&device)
	if result.Error == gorm.ErrRecordNotFound {
		gecho.NotFound(w).WithMessage(fmt.Sprintf("No device with %s of '%s'", idType, idStr)).Send()
		return
	}
	if result.Error != nil {
		gecho.InternalServerError(w).Send()
		logger.Err(result.Error.Error())
		return
	}

	deviceInfo := toDeviceInfo(device)

	gecho.Success(w).WithData(deviceInfo).Send()
}

// DeleteDeviceById
//
// @Summary		Delete device by id
// @Description	Delete a device from the database by using its id or room. The websocket connection, if present, will also be terminated.
// @Tags			device requiresAuth requiresAdmin
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Device ID or Room"
// @Param			type	query		string	false	"Specify identifier type" Enums("id","room") default("id")
// @Success		204 {object}	apiResponses.BaseBase
// @Failure		401	{object}	apiResponses.UnauthorizedError
// @Failure		403	{object}	apiResponses.ForbiddenError
// @Failure		404	{object}	apiResponses.NotFoundError
// @Failure		500	{object}	apiResponses.InternalServerError
// @Router			/device/{id} [delete]
func (h *DeviceHandler) DeleteDeviceById(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodDelete); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}
	ctx := r.Context()

	query := r.URL.Query()
	dbQuery := h.db.Model(&models.Device{})

	idStr := r.PathValue("id")
	idType := query.Get("type")
	if idType == "" {
		idType = "id"
	}

	switch idType {
	case "id":
		userID, err := strconv.ParseUint(idStr, 10, 0)
		if err != nil {
			gecho.BadRequest(w).WithMessage("Invalid device ID, expected positive integer").Send()
			return
		}
		dbQuery = dbQuery.Where("id = ?", userID)
	case "room":
		dbQuery = dbQuery.Where("room = ?", idStr)
	default:
		gecho.BadRequest(w).WithMessage(fmt.Sprintf("Invalid identifier type '%s'", idType)).Send()
		return
	}

	var device models.Device
	result := dbQuery.First(&device)
	if result.Error == gorm.ErrRecordNotFound {
		gecho.NotFound(w).WithMessage(fmt.Sprintf("No device with %s of '%s'", idType, idStr)).Send()
		return
	}
	if result.Error != nil {
		gecho.InternalServerError(w).Send()
		logger.Err(result.Error.Error())
		return
	}

	connID, ok := h.websocketHandler.connectedDevices[device.ID]
	if ok {
		conn, ok := h.websocketHandler.connections[device.ID]
		if ok {
			sendMessage(conn.ws, map[string]any{
				"e":    4,
				"info": "Device deleted.",
			})
			conn.close()
		} else {
			logger.Err(fmt.Sprintf("Tried to terminate connection for device %d but connection %d does not exist.", device.ID, connID))
		}
	}

	rows, err := gorm.G[models.Device](h.db).Where("id = ?", device.ID).Delete(ctx)
	if err != nil {
		logger.Err(err)
		gecho.InternalServerError(w).WithMessage("Failed to delete from database. Any active connection was terminated.").Send()
	}
	if rows > 1 {
		logger.Err(fmt.Sprintf("Deleted %d devices instead of 1 from database!!!!", rows))
		gecho.InternalServerError(w).Send()
		h.quitCh <- os.Interrupt
	}

	gecho.NewErr(w).WithStatus(http.StatusNoContent).Send()
}

// ===== DEVICE REGISTRATION AND RELINKING =====
type PostDeviceRegisterBody struct {
	Pin uint `json:"pin"`
}
type PostDeviceRegisterResponse struct {
	DeviceID uint `json:"device_id"`
}

// PostDeviceRegister
//
// @Summary		Register a new device
// @Description	Register a new device using the registration pin
// @Tags			device requiresAuth requiresAdmin
// @Accept			json
// @Produce		json
// @Param			registration_data	body		PostDeviceRegisterBody	true	"Registration pin\n`pin`: 4 digit registration pin recieved by the device via websocket API"
// @Success		200	{object}	apiResponses.BaseResponse{data=PostDeviceRegisterResponse}
// @Failure		404	{object}	apiResponses.NotFoundError
// @Failure		500	{object}	apiResponses.InternalServerError
// @Router			/device/register [post]
func (h *DeviceHandler) PostDeviceRegister(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodPost); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}
	var body PostDeviceRegisterBody

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		gecho.BadRequest(w).WithMessage(err.Error()).Send()
		logger.Err(err)
		return
	}
	device, err := h.websocketHandler.registerWithPin(body.Pin, nil)
	if err != nil {
		if err.Error() == "No connectionID for this pin" {
			gecho.BadRequest(w).WithMessage("Invalid pin").Send()
		} else {
			gecho.InternalServerError(w).WithMessage(err.Error()).Send()
		}
		return
	}

	RegistrationPinData := PostDeviceRegisterResponse{
		DeviceID: device.ID,
	}

	gecho.Created(w).WithData(RegistrationPinData).Send()
}

type PostDeviceRelinkBody struct {
	Pin      uint `json:"pin"`
	DeviceID uint `json:"device_id"`
}

// PostDeviceRegister
//
// @Summary		Relink a device to an old database entry
// @Description	Relink a device using the registration pin. WARNING: This will generate a new auth token for the device.
// @Tags			device requiresAuth requiresAdmin
// @Accept			json
// @Produce		json
// @Param			registration_data	body		PostDeviceRelinkBody	true	"Registration pin and device ID\n`pin`: 4 digit registration pin recieved by the device via websocket API"
// @Success		200	{object}	apiResponses.BaseResponse{data=PostDeviceRegisterResponse}
// @Failure		404	{object}	apiResponses.NotFoundError
// @Failure		500	{object}	apiResponses.InternalServerError
// @Router			/device/relink [post]
func (h *DeviceHandler) PostDeviceRelink(w http.ResponseWriter, r *http.Request) {
	if err := gecho.Handlers.HandleMethod(w, r, http.MethodPost); err != nil {
		err.Send() // Automatically sends 405 Method Not Allowed
		return
	}
	ctx := r.Context()
	var body PostDeviceRelinkBody

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		gecho.BadRequest(w).WithMessage(err.Error()).Send()
		logger.Err(err)
		return
	}

	deviceFromDb, err := gorm.G[models.Device](h.db).Where("id = ?", body.DeviceID).First(ctx)
	if err == gorm.ErrRecordNotFound {
		gecho.NotFound(w).WithMessage(fmt.Sprintf("No device with id of %d", body.DeviceID)).Send()
		return
	}
	if err != nil {
		gecho.InternalServerError(w).Send()
		logger.Err(err.Error())
		return
	}
	device := &deviceFromDb

	device, err = h.websocketHandler.registerWithPin(body.Pin, device)
	if err != nil {
		if err.Error() == "No connectionID for this pin" {
			gecho.BadRequest(w).WithMessage("Invalid pin").Send()
		} else {
			gecho.InternalServerError(w).WithMessage(err.Error()).Send()
		}
		return
	}

	RegistrationPinData := PostDeviceRegisterResponse{
		DeviceID: device.ID,
	}

	gecho.Created(w).WithData(RegistrationPinData).Send()
}
