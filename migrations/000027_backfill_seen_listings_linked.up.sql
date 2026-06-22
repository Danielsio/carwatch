-- Migrate orphaned seen_listings rows for already-linked accounts.
-- When LinkTelegramToWeb ran before the seen_listings fix, dedup
-- entries remained under the web user's chat_id, causing the scheduler
-- to re-deliver already-seen listings under the telegram chat_id.

-- Step 1: Move web user's seen_listings to the linked telegram user,
-- skipping rows that already exist under the telegram chat_id.
UPDATE seen_listings sl
SET chat_id = u.chat_id
FROM users u
WHERE u.linked_web_id = sl.chat_id
  AND u.channel = 'telegram'
  AND NOT EXISTS (
      SELECT 1 FROM seen_listings s2
      WHERE s2.chat_id = u.chat_id AND s2.token = sl.token
  );

-- Step 2: Delete any remaining orphaned web-user rows that couldn't
-- be migrated (duplicate token already existed under telegram chat_id).
DELETE FROM seen_listings sl
USING users u
WHERE u.linked_web_id IS NOT NULL
  AND u.channel = 'telegram'
  AND sl.chat_id = u.linked_web_id;
