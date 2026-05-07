-- Revert the application role to its initial NOLOGIN state. The
-- password is cleared so a future re-up doesn't inherit a stale
-- credential.
ALTER ROLE app WITH NOLOGIN PASSWORD NULL;
