"use client";

import Image from "next/image";
import { cn } from "@/lib/utils";

interface FloatingCharacterProps {
  src: string;
  size?: number;
  delay?: number;
  className?: string;
  alt?: string;
}

export function FloatingCharacter({
  src,
  size = 120,
  delay = 0,
  className,
  alt = "character",
}: FloatingCharacterProps) {
  return (
    <div
      className={cn("animate-float pointer-events-none select-none", className)}
      style={{ animationDelay: `${delay}s`, width: size, height: size }}
    >
      <Image
        src={src}
        alt={alt}
        width={size}
        height={size}
        className="h-full w-full object-contain drop-shadow-lg"
      />
    </div>
  );
}
