import { NextRequest, NextResponse } from "next/server";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

/**
 * Twitter OAuth callback handler.
 * Twitter redirects here with ?code=xxx&state=yyy.
 * We exchange the code via the backend, then redirect to the dashboard with the token.
 */
export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url);
  const code = searchParams.get("code");
  const state = searchParams.get("state");

  if (!code || !state) {
    return NextResponse.redirect(new URL("/en?error=missing_code", request.url));
  }

  try {
    // Exchange code with backend
    const res = await fetch(`${API_BASE}/api/v1/auth/twitter/callback`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code, state }),
    });

    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      console.error("Twitter callback error:", res.status, body);
      return NextResponse.redirect(new URL("/en/dashboard?error=auth_failed", request.url));
    }

    const data = await res.json();

    // Redirect to dashboard with token in hash (not query param — safer)
    const dashboardURL = new URL("/en/dashboard", request.url);
    dashboardURL.hash = `token=${data.token}&handle=${data.twitter_handle}`;
    return NextResponse.redirect(dashboardURL);
  } catch (err) {
    console.error("Twitter callback fetch error:", err);
    return NextResponse.redirect(new URL("/en/dashboard?error=network_error", request.url));
  }
}
