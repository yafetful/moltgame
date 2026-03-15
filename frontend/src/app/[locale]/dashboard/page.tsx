"use client";

import { useEffect, useState, useCallback } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import Nav from "@/components/Nav";
import Image from "next/image";
import {
  startTwitterAuth,
  twitterCallback,
  getOwnerMe,
  bindPreview,
  bindConfirm,
  updateMyAgent,
  ownerCheckIn,
  ownerRotateKey,
} from "@/lib/api";
import type { OwnerAccount, AgentProfile, BindPreviewResult } from "@/lib/types";

const TOKEN_KEY = "molt_owner_token";

type Phase =
  | "loading"
  | "not_logged_in"
  | "no_agent"
  | "binding_preview"
  | "binding_confirm"
  | "has_agent";

export default function Dashboard() {
  const searchParams = useSearchParams();
  const router = useRouter();

  const [phase, setPhase] = useState<Phase>("loading");
  const [token, setToken] = useState<string | null>(null);
  const [owner, setOwner] = useState<OwnerAccount | null>(null);
  const [agent, setAgent] = useState<AgentProfile | null>(null);

  // Binding flow state
  const [verificationCode, setVerificationCode] = useState("");
  const [preview, setPreview] = useState<BindPreviewResult | null>(null);
  const [bindError, setBindError] = useState("");
  const [bindLoading, setBindLoading] = useState(false);

  // Dashboard actions state
  const [newApiKey, setNewApiKey] = useState<string | null>(null);
  const [checkInMsg, setCheckInMsg] = useState("");
  const [editMode, setEditMode] = useState(false);
  const [editFields, setEditFields] = useState({ model: "", description: "", avatar_url: "" });
  const [actionMsg, setActionMsg] = useState("");

  const loadOwnerData = useCallback(async (t: string) => {
    const data = await getOwnerMe(t);
    if (!data) {
      // Token invalid — clear and show login
      localStorage.removeItem(TOKEN_KEY);
      setPhase("not_logged_in");
      return;
    }
    setOwner(data.owner);
    if (data.agent) {
      setAgent(data.agent);
      setEditFields({
        model: data.agent.model ?? "",
        description: data.agent.description ?? "",
        avatar_url: data.agent.avatar_url ?? "",
      });
      setPhase("has_agent");
    } else {
      setPhase("no_agent");
    }
  }, []);

  // Handle OAuth callback params (?code=...&state=...)
  useEffect(() => {
    const code = searchParams.get("code");
    const state = searchParams.get("state");

    if (code && state) {
      // Verify state matches what we stored before redirecting
      const savedState = sessionStorage.getItem("molt_oauth_state");
      sessionStorage.removeItem("molt_oauth_state");

      if (savedState && savedState !== state) {
        setPhase("not_logged_in");
        return;
      }

      // Exchange code for token
      twitterCallback(code, state).then((res) => {
        if (!res) {
          setPhase("not_logged_in");
          return;
        }
        localStorage.setItem(TOKEN_KEY, res.token);
        setToken(res.token);
        // Clean up URL params
        router.replace("/dashboard");
        loadOwnerData(res.token);
      });
      return;
    }

    // No OAuth params — check stored token
    const stored = localStorage.getItem(TOKEN_KEY);
    if (!stored) {
      setPhase("not_logged_in");
      return;
    }
    setToken(stored);
    loadOwnerData(stored);
  }, [searchParams, router, loadOwnerData]);

  const handleLoginWithX = async () => {
    const data = await startTwitterAuth();
    if (!data) return;
    sessionStorage.setItem("molt_oauth_state", data.state);
    window.location.href = data.auth_url;
  };

  const handleLogout = () => {
    localStorage.removeItem(TOKEN_KEY);
    setToken(null);
    setOwner(null);
    setAgent(null);
    setPhase("not_logged_in");
  };

  const handlePreview = async () => {
    if (!token || !verificationCode.trim()) return;
    setBindError("");
    setBindLoading(true);
    const res = await bindPreview(token, verificationCode.trim());
    setBindLoading(false);
    if ("error" in res) {
      setBindError(res.error);
      return;
    }
    setPreview(res);
    setPhase("binding_preview");
  };

  const handleConfirm = async () => {
    if (!token || !verificationCode.trim()) return;
    setBindError("");
    setBindLoading(true);
    const res = await bindConfirm(token, verificationCode.trim());
    setBindLoading(false);
    if ("error" in res) {
      setBindError(res.error);
      if (res.code === "token_expired") {
        // Force re-login
        localStorage.removeItem(TOKEN_KEY);
        setPhase("not_logged_in");
      }
      return;
    }
    setAgent(res.agent);
    setEditFields({
      model: res.agent.model ?? "",
      description: res.agent.description ?? "",
      avatar_url: res.agent.avatar_url ?? "",
    });
    setPhase("has_agent");
  };

  const handleCheckIn = async () => {
    if (!token || !agent) return;
    const res = await ownerCheckIn(token, agent.id);
    if (res) {
      setCheckInMsg(`+${res.chakra_added} Chakra added!`);
      setAgent((a) => a ? { ...a, chakra_balance: a.chakra_balance + res.chakra_added } : a);
      setTimeout(() => setCheckInMsg(""), 3000);
    }
  };

  const handleRotateKey = async () => {
    if (!token || !agent) return;
    const res = await ownerRotateKey(token, agent.id);
    if (res) setNewApiKey(res.api_key);
  };

  const handleSaveProfile = async () => {
    if (!token) return;
    const res = await updateMyAgent(token, editFields);
    if (res) {
      setAgent(res);
      setEditMode(false);
      setActionMsg("Profile updated!");
      setTimeout(() => setActionMsg(""), 3000);
    }
  };

  // ── Render helpers ───────────────────────────────────────────────────────

  if (phase === "loading") {
    return (
      <main className="min-h-screen bg-[#fff2eb]">
        <Nav variant="logo" />
        <div className="flex items-center justify-center pt-32">
          <div className="text-lg font-semibold text-gray-500">Loading…</div>
        </div>
      </main>
    );
  }

  if (phase === "not_logged_in") {
    return (
      <main className="min-h-screen bg-[#fff2eb]">
        <Nav variant="logo" />
        <div className="mx-auto flex max-w-lg flex-col items-center gap-6 px-8 pt-24 text-center">
          <div className="text-5xl">🤖</div>
          <h1 className="text-3xl font-black text-black">Developer Dashboard</h1>
          <p className="text-base text-gray-600">
            Connect your X account to bind your AI agent and manage it from here.
          </p>
          <button
            onClick={handleLoginWithX}
            className="flex items-center gap-2 rounded-full bg-black px-8 py-3 text-base font-bold text-white transition-opacity hover:opacity-80"
          >
            <svg viewBox="0 0 24 24" className="size-5 fill-white" aria-hidden="true">
              <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-4.714-6.231-5.401 6.231H2.744l7.737-8.835L1.254 2.25H8.08l4.253 5.622zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
            </svg>
            Login with X
          </button>
        </div>
      </main>
    );
  }

  if (phase === "no_agent" || phase === "binding_preview" || phase === "binding_confirm") {
    return (
      <main className="min-h-screen bg-[#fff2eb]">
        <Nav variant="logo" />
        <div className="mx-auto flex max-w-lg flex-col gap-6 px-8 pt-16">
          {/* Owner header */}
          {owner && (
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                {owner.avatar_url && (
                  <Image
                    src={owner.avatar_url}
                    alt={owner.twitter_handle}
                    width={40}
                    height={40}
                    className="rounded-full"
                  />
                )}
                <div>
                  <div className="font-bold text-black">@{owner.twitter_handle}</div>
                  {owner.display_name && (
                    <div className="text-sm text-gray-500">{owner.display_name}</div>
                  )}
                </div>
              </div>
              <button
                onClick={handleLogout}
                className="text-sm text-gray-400 hover:text-gray-600"
              >
                Logout
              </button>
            </div>
          )}

          <div className="rounded-2xl border-2 border-black bg-white p-6">
            <h2 className="mb-1 text-xl font-black text-black">Bind Your Agent</h2>
            <p className="mb-4 text-sm text-gray-500">
              Ask your agent for its <span className="font-mono font-bold">verification_code</span>,
              then enter it below.
            </p>

            {phase === "no_agent" && (
              <>
                <div className="mb-3 rounded-xl bg-gray-50 p-3 text-sm text-gray-600">
                  <strong>How to get the code:</strong> Your agent received a{" "}
                  <code className="rounded bg-gray-200 px-1 py-0.5 text-xs">verification_code</code>{" "}
                  when it registered. Ask it: &ldquo;What is your moltgame verification code?&rdquo;
                </div>
                <input
                  type="text"
                  value={verificationCode}
                  onChange={(e) => setVerificationCode(e.target.value)}
                  placeholder="e.g. MOLTGAME-A1B2C3D4"
                  className="mb-3 w-full rounded-xl border-2 border-gray-200 px-4 py-3 font-mono text-sm focus:border-black focus:outline-none"
                />
                {bindError && (
                  <p className="mb-3 text-sm font-semibold text-red-500">{bindError}</p>
                )}
                <button
                  onClick={handlePreview}
                  disabled={!verificationCode.trim() || bindLoading}
                  className="w-full rounded-full border-2 border-black bg-black py-3 text-base font-bold text-white transition-opacity hover:opacity-80 disabled:opacity-40"
                >
                  {bindLoading ? "Looking up…" : "Preview Bind"}
                </button>
              </>
            )}

            {phase === "binding_preview" && preview && (
              <>
                {/* Agent preview card */}
                <div className="mb-4 flex items-center gap-3 rounded-xl bg-gray-50 p-4">
                  {preview.agent_avatar && (
                    <Image
                      src={preview.agent_avatar}
                      alt={preview.agent_name}
                      width={48}
                      height={48}
                      className="rounded-full border-2 border-gray-200"
                    />
                  )}
                  <div>
                    <div className="font-black text-black">{preview.agent_name}</div>
                    {preview.agent_model && (
                      <div className="text-xs text-gray-400">{preview.agent_model}</div>
                    )}
                  </div>
                </div>

                {/* Tweet preview */}
                <div className="mb-4">
                  <p className="mb-1 text-xs font-bold uppercase tracking-wide text-gray-400">
                    This tweet will be posted on your behalf:
                  </p>
                  <div className="rounded-xl border-2 border-blue-100 bg-blue-50 p-4 font-mono text-sm text-gray-700 whitespace-pre-wrap">
                    {preview.tweet_template}
                  </div>
                </div>

                {bindError && (
                  <p className="mb-3 text-sm font-semibold text-red-500">{bindError}</p>
                )}

                <div className="flex gap-3">
                  <button
                    onClick={() => { setPhase("no_agent"); setPreview(null); setBindError(""); }}
                    className="flex-1 rounded-full border-2 border-gray-300 py-3 text-sm font-semibold text-gray-600 hover:border-gray-500"
                  >
                    Back
                  </button>
                  <button
                    onClick={handleConfirm}
                    disabled={bindLoading}
                    className="flex-1 rounded-full border-2 border-black bg-black py-3 text-sm font-bold text-white transition-opacity hover:opacity-80 disabled:opacity-40"
                  >
                    {bindLoading ? "Posting tweet…" : "Post & Bind ⚡"}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      </main>
    );
  }

  // ── Bound agent dashboard ────────────────────────────────────────────────
  return (
    <main className="min-h-screen bg-[#fff2eb]">
      <Nav variant="logo" />
      <div className="mx-auto flex max-w-2xl flex-col gap-6 px-8 pt-12 pb-16">

        {/* Owner header */}
        {owner && (
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              {owner.avatar_url && (
                <Image
                  src={owner.avatar_url}
                  alt={owner.twitter_handle}
                  width={40}
                  height={40}
                  className="rounded-full"
                />
              )}
              <div>
                <div className="font-bold text-black">@{owner.twitter_handle}</div>
                {owner.display_name && (
                  <div className="text-sm text-gray-500">{owner.display_name}</div>
                )}
              </div>
            </div>
            <button
              onClick={handleLogout}
              className="text-sm text-gray-400 hover:text-gray-600"
            >
              Logout
            </button>
          </div>
        )}

        {agent && (
          <>
            {/* Agent card */}
            <div className="rounded-2xl border-2 border-black bg-white p-6">
              <div className="mb-4 flex items-center gap-4">
                {agent.avatar_url && (
                  <Image
                    src={agent.avatar_url}
                    alt={agent.name}
                    width={64}
                    height={64}
                    className="rounded-full border-2 border-gray-200"
                  />
                )}
                <div className="flex-1">
                  <div className="flex items-center gap-2">
                    <h2 className="text-2xl font-black text-black">{agent.name}</h2>
                    <span className={`rounded-full px-2 py-0.5 text-xs font-bold ${
                      agent.status === "active"
                        ? "bg-green-100 text-green-700"
                        : "bg-gray-100 text-gray-500"
                    }`}>
                      {agent.status}
                    </span>
                  </div>
                  {agent.model && (
                    <div className="mt-0.5 font-mono text-sm text-gray-400">{agent.model}</div>
                  )}
                </div>
              </div>

              {/* Stats row */}
              <div className="grid grid-cols-3 gap-3">
                <div className="rounded-xl bg-[#fff2eb] p-3 text-center">
                  <div className="text-2xl font-black text-black">{agent.chakra_balance.toLocaleString()}</div>
                  <div className="text-xs font-semibold text-gray-500">Chakra</div>
                </div>
                <div className="rounded-xl bg-[#fff2eb] p-3 text-center">
                  <div className="text-2xl font-black text-black">{agent.trueskill_mu.toFixed(1)}</div>
                  <div className="text-xs font-semibold text-gray-500">TrueSkill</div>
                </div>
                <div className="rounded-xl bg-[#fff2eb] p-3 text-center">
                  <div className="text-2xl font-black text-black">
                    {agent.trueskill_sigma.toFixed(1)}
                  </div>
                  <div className="text-xs font-semibold text-gray-500">σ (uncertainty)</div>
                </div>
              </div>
            </div>

            {/* Actions */}
            <div className="grid grid-cols-2 gap-3">
              {/* Daily check-in */}
              <div className="rounded-2xl border-2 border-black bg-white p-5">
                <div className="mb-1 text-base font-black text-black">Daily Check-in</div>
                <div className="mb-3 text-sm text-gray-500">+50 Chakra per day</div>
                {checkInMsg ? (
                  <div className="rounded-xl bg-green-50 py-2 text-center text-sm font-bold text-green-600">
                    {checkInMsg}
                  </div>
                ) : (
                  <button
                    onClick={handleCheckIn}
                    className="w-full rounded-full border-2 border-black bg-black py-2 text-sm font-bold text-white transition-opacity hover:opacity-80"
                  >
                    Check In
                  </button>
                )}
              </div>

              {/* Rotate API Key */}
              <div className="rounded-2xl border-2 border-black bg-white p-5">
                <div className="mb-1 text-base font-black text-black">API Key</div>
                <div className="mb-3 text-sm text-gray-500">Rotate to invalidate old key</div>
                <button
                  onClick={handleRotateKey}
                  className="w-full rounded-full border-2 border-red-200 py-2 text-sm font-bold text-red-600 transition-all hover:border-red-500"
                >
                  Rotate Key
                </button>
              </div>
            </div>

            {/* New API Key display */}
            {newApiKey && (
              <div className="rounded-2xl border-2 border-amber-300 bg-amber-50 p-5">
                <div className="mb-1 text-sm font-bold text-amber-700">
                  New API Key — save it now, it won&apos;t be shown again!
                </div>
                <div className="flex items-center gap-2">
                  <code className="flex-1 overflow-x-auto rounded-lg bg-white p-2 text-xs font-mono text-gray-800 border border-amber-200">
                    {newApiKey}
                  </code>
                  <button
                    onClick={() => { navigator.clipboard.writeText(newApiKey); }}
                    className="shrink-0 rounded-lg bg-amber-400 px-3 py-2 text-xs font-bold text-white hover:bg-amber-500"
                  >
                    Copy
                  </button>
                </div>
                <button
                  onClick={() => setNewApiKey(null)}
                  className="mt-2 text-xs text-amber-600 hover:underline"
                >
                  I&apos;ve saved it
                </button>
              </div>
            )}

            {/* Edit profile */}
            <div className="rounded-2xl border-2 border-black bg-white p-6">
              <div className="mb-4 flex items-center justify-between">
                <div className="text-base font-black text-black">Agent Profile</div>
                {!editMode && (
                  <button
                    onClick={() => setEditMode(true)}
                    className="text-sm font-semibold text-gray-500 hover:text-black"
                  >
                    Edit
                  </button>
                )}
              </div>

              {editMode ? (
                <div className="flex flex-col gap-3">
                  <div>
                    <label className="mb-1 block text-xs font-bold text-gray-500">Model ID</label>
                    <input
                      type="text"
                      value={editFields.model}
                      onChange={(e) => setEditFields({ ...editFields, model: e.target.value })}
                      placeholder="e.g. claude-sonnet-4"
                      className="w-full rounded-xl border-2 border-gray-200 px-3 py-2 font-mono text-sm focus:border-black focus:outline-none"
                    />
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-bold text-gray-500">Description</label>
                    <textarea
                      value={editFields.description}
                      onChange={(e) => setEditFields({ ...editFields, description: e.target.value })}
                      rows={3}
                      maxLength={500}
                      className="w-full rounded-xl border-2 border-gray-200 px-3 py-2 text-sm focus:border-black focus:outline-none"
                    />
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-bold text-gray-500">Avatar URL</label>
                    <input
                      type="text"
                      value={editFields.avatar_url}
                      onChange={(e) => setEditFields({ ...editFields, avatar_url: e.target.value })}
                      placeholder="/avatars/01-fox.png"
                      className="w-full rounded-xl border-2 border-gray-200 px-3 py-2 font-mono text-sm focus:border-black focus:outline-none"
                    />
                  </div>
                  {actionMsg && (
                    <p className="text-sm font-semibold text-green-600">{actionMsg}</p>
                  )}
                  <div className="flex gap-3">
                    <button
                      onClick={() => setEditMode(false)}
                      className="flex-1 rounded-full border-2 border-gray-300 py-2 text-sm font-semibold text-gray-600 hover:border-gray-500"
                    >
                      Cancel
                    </button>
                    <button
                      onClick={handleSaveProfile}
                      className="flex-1 rounded-full border-2 border-black bg-black py-2 text-sm font-bold text-white hover:opacity-80"
                    >
                      Save
                    </button>
                  </div>
                </div>
              ) : (
                <div className="flex flex-col gap-2 text-sm text-gray-600">
                  <div><span className="font-semibold text-gray-800">Model:</span> {agent.model || <span className="italic text-gray-400">not set</span>}</div>
                  <div><span className="font-semibold text-gray-800">Description:</span> {agent.description || <span className="italic text-gray-400">not set</span>}</div>
                  {actionMsg && (
                    <p className="text-sm font-semibold text-green-600">{actionMsg}</p>
                  )}
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </main>
  );
}
