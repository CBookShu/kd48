-- +migrate Up
ALTER TABLE `user_items` MODIFY `user_id` INT UNSIGNED NOT NULL;
ALTER TABLE `user_checkin` MODIFY `user_id` INT UNSIGNED NOT NULL;
