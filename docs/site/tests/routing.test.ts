import { expect, test, describe } from 'vitest';
import { cleanPath, buildNavLinks } from '../src/utils/routing';

describe('routing', () => {
  describe('cleanPath', () => {
    test('given ../../../../README.md returns README', () => {
      expect(cleanPath('../../../../README.md')).toBe('README');
    });

    test('given ../../../../docs/adr/0001-record.md returns adr/0001-record', () => {
      expect(cleanPath('../../../../docs/adr/0001-record.md')).toBe('adr/0001-record');
    });
  });

  describe('buildNavLinks', () => {
    test('sorts ADRs at the bottom, capitalizes titles, and sets isAdr', () => {
      const keys = [
        '../../../../docs/adr/0002-second.md',
        '../../../../docs/setup.md',
        '../../../../docs/adr/0001-record.md',
        '../../../../README.md',
      ];
      const navLinks = buildNavLinks(keys, 'readme');

      expect(navLinks.length).toBe(4);
      
      expect(navLinks[0].title).toBe('README');
      expect(navLinks[0].isAdr).toBe(false);
      
      expect(navLinks[1].title).toBe('Setup');
      expect(navLinks[1].isAdr).toBe(false);
      
      expect(navLinks[2].title).toBe('0001 Record');
      expect(navLinks[2].isAdr).toBe(true);
      
      expect(navLinks[3].title).toBe('0002 Second');
      expect(navLinks[3].isAdr).toBe(true);
    });
  });
});