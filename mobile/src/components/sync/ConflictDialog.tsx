import React, { useState, useMemo } from 'react';
import {
  View,
  Text,
  Modal,
  TouchableOpacity,
  ScrollView,
  StyleSheet,
  SafeAreaView,
} from 'react-native';
import { ResolutionStrategy, SyncConflict, ResolvedData } from '@/types/conflict';
import { colors, spacing, FontSize } from '@/utils/theme';

interface FieldChoice {
  fieldName: string;
  choice: 'local' | 'server' | 'merge';
}

interface ConflictDialogProps {
  visible: boolean;
  conflict: SyncConflict | null;
  onResolve: (resolvedData: ResolvedData) => void;
  onDismiss: () => void;
  fieldLabels?: Record<string, string>;
}

const ConflictDialog: React.FC<ConflictDialogProps> = ({
  visible,
  conflict,
  onResolve,
  onDismiss,
  fieldLabels = {},
}) => {
  const [fieldChoices, setFieldChoices] = useState<Record<string, 'local' | 'server' | 'merge'>>({});

  const conflictFields = useMemo(() => {
    if (!conflict?.field_conflicts) return [];
    return conflict.field_conflicts;
  }, [conflict]);

  const allFields = useMemo(() => {
    if (!conflict) return [];
    return [...new Set([...Object.keys(conflict.local_value), ...Object.keys(conflict.server_value)])];
  }, [conflict]);

  const handleFieldChoice = (fieldName: string, choice: 'local' | 'server' | 'merge') => {
    setFieldChoices((prev) => ({
      ...prev,
      [fieldName]: choice,
    }));
  };

  const handleResolveAllLocal = () => {
    const choices: Record<string, 'local' | 'server' | 'merge'> = {};
    conflictFields.forEach((fc) => {
      choices[fc.field_name] = 'local';
    });
    setFieldChoices(choices);
  };

  const handleResolveAllServer = () => {
    const choices: Record<string, 'local' | 'server' | 'merge'> = {};
    conflictFields.forEach((fc) => {
      choices[fc.field_name] = 'server';
    });
    setFieldChoices(choices);
  };

  const handleConfirm = () => {
    if (!conflict) return;

    const resolvedData: ResolvedData = {
      conflict_id: conflict.id,
      entity_type: conflict.entity_type,
      entity_id: conflict.entity_id,
      resolved_value: {},
      strategy_used: ResolutionStrategy.MANUAL,
      resolved_at: new Date().toISOString(),
    };

    const resolvedValue: Record<string, any> = {};
    const fieldResolutions = conflictFields.map((fc) => {
      const choice = fieldChoices[fc.field_name] || 'server';
      const resolvedFieldValue = choice === 'local' ? fc.local_value : fc.server_value;

      resolvedValue[fc.field_name] = resolvedFieldValue;

      return {
        field_name: fc.field_name,
        local_value: fc.local_value,
        server_value: fc.server_value,
        resolved_value: resolvedFieldValue,
        strategy_used: choice === 'local' ? ResolutionStrategy.CLIENT_WINS : ResolutionStrategy.SERVER_WINS,
      };
    });

    allFields.forEach((field) => {
      if (!resolvedValue[field]) {
        resolvedValue[field] = conflict.local_value[field] ?? conflict.server_value[field];
      }
    });

    resolvedData.resolved_value = resolvedValue;
    resolvedData.field_resolutions = fieldResolutions;

    onResolve(resolvedData);
    setFieldChoices({});
  };

  const handleDismiss = () => {
    setFieldChoices({});
    onDismiss();
  };

  const formatValue = (value: any): string => {
    if (value === null || value === undefined) return '(empty)';
    if (typeof value === 'boolean') return value ? 'Yes' : 'No';
    if (typeof value === 'object') return JSON.stringify(value);
    return String(value);
  };

  const getFieldLabel = (fieldName: string): string => {
    return fieldLabels[fieldName] || fieldName.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase());
  };

  const getEntityTypeLabel = (entityType: string): string => {
    const labels: Record<string, string> = {
      shipment: 'Shipment',
      tracking_event: 'Tracking Event',
      proof_of_delivery: 'Proof of Delivery',
    };
    return labels[entityType] || entityType;
  };

  if (!conflict) return null;

  return (
    <Modal visible={visible} animationType="slide" transparent={false} onRequestClose={handleDismiss}>
      <SafeAreaView style={styles.container}>
        <View style={styles.header}>
          <TouchableOpacity onPress={handleDismiss} style={styles.closeButton}>
            <Text style={styles.closeButtonText}>Cancel</Text>
          </TouchableOpacity>
          <Text style={styles.title}>Resolve Conflict</Text>
          <TouchableOpacity onPress={handleConfirm} style={styles.confirmButton}>
            <Text style={styles.confirmButtonText}>Apply</Text>
          </TouchableOpacity>
        </View>

        <View style={styles.infoBanner}>
          <Text style={styles.infoText}>
            {getEntityTypeLabel(conflict.entity_type)} #{conflict.entity_id.slice(0, 8)} has conflicting changes
          </Text>
        </View>

        <View style={styles.quickActions}>
          <TouchableOpacity style={styles.quickActionButton} onPress={handleResolveAllServer}>
            <Text style={styles.quickActionText}>Use All Server</Text>
          </TouchableOpacity>
          <TouchableOpacity style={styles.quickActionButton} onPress={handleResolveAllLocal}>
            <Text style={styles.quickActionText}>Use All Local</Text>
          </TouchableOpacity>
        </View>

        <ScrollView style={styles.scrollView} contentContainerStyle={styles.scrollContent}>
          <View style={styles.columnsHeader}>
            <Text style={styles.columnHeader}>Local</Text>
            <Text style={styles.columnHeader}>Server</Text>
          </View>

          {conflictFields.map((fieldConflict) => {
            const choice = fieldChoices[fieldConflict.field_name] || 'server';

            return (
              <View key={fieldConflict.field_name} style={styles.fieldRow}>
                <Text style={styles.fieldLabel}>{getFieldLabel(fieldConflict.field_name)}</Text>

                <View style={styles.valuesContainer}>
                  <TouchableOpacity
                    style={[styles.valueBox, choice === 'local' && styles.valueBoxSelected]}
                    onPress={() => handleFieldChoice(fieldConflict.field_name, 'local')}
                  >
                    <Text style={[styles.valueText, choice === 'local' && styles.valueTextSelected]}>
                      {formatValue(fieldConflict.local_value)}
                    </Text>
                    {choice === 'local' && <Text style={styles.selectedIndicator}>Selected</Text>}
                  </TouchableOpacity>

                  <TouchableOpacity
                    style={[styles.valueBox, choice === 'server' && styles.valueBoxSelected]}
                    onPress={() => handleFieldChoice(fieldConflict.field_name, 'server')}
                  >
                    <Text style={[styles.valueText, choice === 'server' && styles.valueTextSelected]}>
                      {formatValue(fieldConflict.server_value)}
                    </Text>
                    {choice === 'server' && <Text style={styles.selectedIndicator}>Selected</Text>}
                  </TouchableOpacity>
                </View>
              </View>
            );
          })}

          {conflictFields.length === 0 && (
            <View style={styles.noConflictsContainer}>
              <Text style={styles.noConflictsText}>No conflicts to display</Text>
            </View>
          )}
        </ScrollView>

        <View style={styles.footer}>
          <Text style={styles.footerText}>
            {conflictFields.length} field{conflictFields.length !== 1 ? 's' : ''} with conflicts
          </Text>
        </View>
      </SafeAreaView>
    </Modal>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.background,
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.md,
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
  },
  closeButton: {
    padding: spacing.sm,
  },
  closeButtonText: {
    color: colors.error,
    fontSize: FontSize.md,
  },
  title: {
    fontSize: FontSize.lg,
    fontWeight: '600',
    color: colors.text,
  },
  confirmButton: {
    padding: spacing.sm,
  },
  confirmButtonText: {
    color: colors.primary,
    fontSize: FontSize.md,
    fontWeight: '600',
  },
  infoBanner: {
    backgroundColor: '#fef3c7',
    paddingVertical: spacing.sm,
    paddingHorizontal: spacing.lg,
  },
  infoText: {
    color: '#92400e',
    fontSize: FontSize.sm,
    textAlign: 'center',
  },
  quickActions: {
    flexDirection: 'row',
    justifyContent: 'center',
    paddingVertical: spacing.md,
    gap: spacing.md,
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
  },
  quickActionButton: {
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.sm,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: colors.primary,
  },
  quickActionText: {
    color: colors.primary,
    fontSize: FontSize.sm,
  },
  scrollView: {
    flex: 1,
  },
  scrollContent: {
    padding: spacing.lg,
  },
  columnsHeader: {
    flexDirection: 'row',
    justifyContent: 'space-around',
    marginBottom: spacing.md,
  },
  columnHeader: {
    fontSize: FontSize.sm,
    fontWeight: '600',
    color: colors.textSecondary,
    flex: 1,
    textAlign: 'center',
  },
  fieldRow: {
    marginBottom: spacing.lg,
  },
  fieldLabel: {
    fontSize: FontSize.md,
    fontWeight: '600',
    color: colors.text,
    marginBottom: spacing.sm,
  },
  valuesContainer: {
    flexDirection: 'row',
    gap: spacing.sm,
  },
  valueBox: {
    flex: 1,
    padding: spacing.md,
    backgroundColor: colors.surface,
    borderRadius: 8,
    borderWidth: 2,
    borderColor: colors.border,
  },
  valueBoxSelected: {
    borderColor: colors.primary,
    backgroundColor: '#dbeafe',
  },
  valueText: {
    fontSize: FontSize.sm,
    color: colors.text,
  },
  valueTextSelected: {
    fontWeight: '600',
  },
  selectedIndicator: {
    fontSize: FontSize.xs,
    color: colors.primary,
    marginTop: spacing.xs,
    fontWeight: '600',
  },
  noConflictsContainer: {
    padding: spacing.xl,
    alignItems: 'center',
  },
  noConflictsText: {
    color: colors.textSecondary,
    fontSize: FontSize.md,
  },
  footer: {
    padding: spacing.lg,
    borderTopWidth: 1,
    borderTopColor: colors.border,
    alignItems: 'center',
  },
  footerText: {
    color: colors.textSecondary,
    fontSize: FontSize.sm,
  },
});

export default ConflictDialog;
