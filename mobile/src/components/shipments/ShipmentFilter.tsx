import { View, Text, StyleSheet, Pressable } from 'react-native';
import { colors, spacing } from '../../utils/theme';

interface ShipmentFilterProps {
  activeFilter: 'active' | 'completed' | 'all';
  onChange: (filter: 'active' | 'completed' | 'all') => void;
}

const FILTERS: Array<{ id: 'active' | 'completed' | 'all'; label: string }> = [
  { id: 'active', label: 'Active' },
  { id: 'completed', label: 'Completed' },
  { id: 'all', label: 'All' },
];

export function ShipmentFilter({ activeFilter, onChange }: ShipmentFilterProps) {
  return (
    <View style={styles.container}>
      {FILTERS.map((filter) => {
        const isActive = activeFilter === filter.id;
        return (
          <Pressable
            key={filter.id}
            style={[styles.tab, isActive && styles.tabActive]}
            onPress={() => onChange(filter.id)}
          >
            <Text style={[styles.tabText, isActive && styles.tabTextActive]}>
              {filter.label}
            </Text>
          </Pressable>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flexDirection: 'row',
    backgroundColor: colors.background,
    borderRadius: 10,
    padding: spacing.xs,
    gap: spacing.xs,
  },
  tab: {
    flex: 1,
    paddingVertical: spacing.sm,
    paddingHorizontal: spacing.md,
    borderRadius: 8,
    alignItems: 'center',
  },
  tabActive: {
    backgroundColor: colors.surface,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.05,
    shadowRadius: 1,
    elevation: 1,
  },
  tabText: {
    fontSize: 14,
    fontWeight: '500',
    color: colors.textSecondary,
  },
  tabTextActive: {
    color: colors.text,
    fontWeight: '600',
  },
});
