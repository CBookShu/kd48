-- +migrate Down
ALTER TABLE `user_items` MODIFY `user_id` BIGINT NOT NULL;
ALTER TABLE `user_checkin` MODIFY `user_id` BIGINT NOT NULL;
