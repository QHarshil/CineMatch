# CineMatch auth email templates

Branded email templates for Supabase Auth, matching the Atlas frontend theme
(white canvas, hairline frame, cornflower blue `#2f54ff`, Georgia serif with a
Courier mono accent). Email clients strip `<link>` fonts and most CSS, so these
use table layout, fully inline styles, and web-safe fonts only.

## magic-link.html

The passwordless sign-in email. Uses Supabase's `{{ .ConfirmationURL }}`
variable, which resolves to the one-time magic link.

### How to apply

Supabase does not read these files automatically. Paste the HTML into the
dashboard:

1. Supabase dashboard > **Authentication** > **Emails** > **Magic Link**.
2. Set the subject to: `Your CineMatch sign-in link`
3. Replace the message body with the contents of `magic-link.html`.
4. Save. Send yourself a link from `/login` to verify rendering.

To apply it as code instead of by hand, set the template under
`auth.email.template.magic_link` in `supabase/config.toml` (Supabase CLI), or
PATCH the Auth config via the Management API
(`GET/PATCH /v1/projects/{ref}/config/auth`, field `mailer_templates_magic_link_content`).

### Notes

- Keep `{{ .ConfirmationURL }}` exactly as written in both the button and the
  fallback link. Supabase substitutes it at send time.
- The magic-link template also covers the OTP / sign-in flow CineMatch uses
  (`signInWithMagicLink` in `frontend/src/lib/auth-context.tsx`).
- If the project later adds confirm-signup or recovery emails, add matching
  templates here using the same shell.
