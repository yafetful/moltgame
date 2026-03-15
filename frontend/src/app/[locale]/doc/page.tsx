import Nav from "@/components/Nav";
import DocContent from "./DocContent";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

async function fetchSkillMd(): Promise<string> {
  try {
    const res = await fetch(`${API_URL}/skill.md`, {
      next: { revalidate: 3600 },
    });
    if (!res.ok) return "";
    return res.text();
  } catch {
    return "";
  }
}

export default async function DocPage() {
  const content = await fetchSkillMd();

  return (
    <div className="min-h-dvh bg-[#fdf6f0]">
      <Nav variant="logo" />
      <DocContent content={content} />
    </div>
  );
}
