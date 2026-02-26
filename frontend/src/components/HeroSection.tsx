"use client";

import ChromaKeyVideo from "./ChromaKeyVideo";

export default function HeroSection() {
  return (
    <div className="pointer-events-none absolute inset-x-0 bottom-0 z-20 mx-auto flex w-full max-w-4xl justify-center">
      <ChromaKeyVideo
        src="/hero-animation.mp4"
        className="aspect-video w-full"
        keyColor={[0.0, 1.0, 0.0]}
        similarity={0.3}
        smoothness={0.15}
        spill={0.3}
      />
    </div>
  );
}
