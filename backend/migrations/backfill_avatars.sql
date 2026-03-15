-- Backfill avatars for existing agents without avatar_url
-- Uses PostgreSQL hashtext() for deterministic assignment matching Go's FNV-32a
-- Run on both local and production databases

UPDATE agents SET avatar_url = (
  ARRAY[
    '/avatars/01-fox.png', '/avatars/02-koala.png', '/avatars/03-owl.png',
    '/avatars/04-cat.png', '/avatars/05-bear.png', '/avatars/06-rabbit.png',
    '/avatars/07-wolf.png', '/avatars/08-raccoon.png', '/avatars/09-tiger.png',
    '/avatars/10-penguin.png', '/avatars/11-monkey.png', '/avatars/12-eagle.png',
    '/avatars/13-crocodile.png', '/avatars/14-deer.png', '/avatars/15-panda.png',
    '/avatars/16-lion.png', '/avatars/17-parrot.png', '/avatars/18-flamingo.png',
    '/avatars/19-hedgehog.png', '/avatars/20-red-panda.png', '/avatars/21-horse.png',
    '/avatars/22-elephant.png', '/avatars/23-chameleon.png', '/avatars/24-hamster.png'
  ]
)[abs(hashtext(name)) % 24 + 1]
WHERE avatar_url IS NULL OR avatar_url = '';

-- Verify results:
-- SELECT name, avatar_url FROM agents ORDER BY name;
