import Image from "next/image";
import { cn } from "@/lib/utils";

interface IconProps {
  size?: number;
  className?: string;
}

export function ChakraIcon({ size = 20, className }: IconProps) {
  return (
    <Image
      src="/icons/chakra.png"
      alt="Chakra"
      width={size}
      height={size}
      className={cn("inline-block", className)}
    />
  );
}

export function PokerIcon({ size = 20, className }: IconProps) {
  return (
    <Image
      src="/icons/poker.png"
      alt="Poker"
      width={size}
      height={size}
      className={cn("inline-block", className)}
    />
  );
}

export function WerewolfIcon({ size = 20, className }: IconProps) {
  return (
    <Image
      src="/icons/werewolves.png"
      alt="Werewolf"
      width={size}
      height={size}
      className={cn("inline-block", className)}
    />
  );
}

export function ChipsIcon({ size = 20, className }: IconProps) {
  return (
    <Image
      src="/icons/chips.png"
      alt="Chips"
      width={size}
      height={size}
      className={cn("inline-block", className)}
    />
  );
}

export function DealerIcon({ size = 20, className }: IconProps) {
  return (
    <Image
      src="/icons/dealer.png"
      alt="Dealer"
      width={size}
      height={size}
      className={cn("inline-block", className)}
    />
  );
}

export function EliminatedIcon({ size = 20, className }: IconProps) {
  return (
    <Image
      src="/icons/eliminated.png"
      alt="Eliminated"
      width={size}
      height={size}
      className={cn("inline-block", className)}
    />
  );
}
