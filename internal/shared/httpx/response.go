package httpx

import "github.com/gofiber/fiber/v3"

func JSONSuccess(c fiber.Ctx, status int, data any) error {
	return c.Status(status).JSON(fiber.Map{
		"success": true,
		"data":    data,
	})
}

func JSONError(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"success": false,
		"error": fiber.Map{
			"message": message,
		},
	})
}
