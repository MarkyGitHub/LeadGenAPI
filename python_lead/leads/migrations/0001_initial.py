# Generated migration for InboundLead and DeliveryAttempt models

from django.db import migrations, models
import django.db.models.deletion


class Migration(migrations.Migration):

    initial = True

    dependencies = [
    ]

    operations = [
        migrations.CreateModel(
            name='InboundLead',
            fields=[
                ('id', models.BigAutoField(auto_created=True, primary_key=True, serialize=False, verbose_name='ID')),
                ('received_at', models.DateTimeField(auto_now_add=True, db_index=True)),
                ('raw_payload', models.JSONField()),
                ('source_headers', models.JSONField(blank=True, null=True)),
                ('status', models.CharField(choices=[('RECEIVED', 'Received'), ('REJECTED', 'Rejected'), ('READY', 'Ready'), ('DELIVERED', 'Delivered'), ('FAILED', 'Failed'), ('PERMANENTLY_FAILED', 'Permanently Failed')], db_index=True, default='RECEIVED', max_length=20)),
                ('rejection_reason', models.CharField(blank=True, max_length=100, null=True)),
                ('normalized_payload', models.JSONField(blank=True, null=True)),
                ('customer_payload', models.JSONField(blank=True, null=True)),
                ('payload_hash', models.CharField(blank=True, db_index=True, max_length=64, null=True)),
                ('created_at', models.DateTimeField(auto_now_add=True)),
                ('updated_at', models.DateTimeField(auto_now=True)),
            ],
            options={
                'ordering': ['-received_at'],
            },
        ),
        migrations.CreateModel(
            name='DeliveryAttempt',
            fields=[
                ('id', models.BigAutoField(auto_created=True, primary_key=True, serialize=False, verbose_name='ID')),
                ('attempt_no', models.PositiveIntegerField()),
                ('requested_at', models.DateTimeField(auto_now_add=True)),
                ('response_status', models.PositiveIntegerField(blank=True, null=True)),
                ('response_body', models.TextField(blank=True, null=True)),
                ('error_message', models.TextField(blank=True, null=True)),
                ('success', models.BooleanField(default=False)),
                ('created_at', models.DateTimeField(auto_now_add=True)),
                ('lead', models.ForeignKey(on_delete=django.db.models.deletion.CASCADE, related_name='delivery_attempts', to='leads.inboundlead')),
            ],
            options={
                'ordering': ['lead', 'attempt_no'],
            },
        ),
        migrations.AddIndex(
            model_name='inboundlead',
            index=models.Index(fields=['status', 'received_at'], name='leads_inbou_status_a1b2c3_idx'),
        ),
        migrations.AddIndex(
            model_name='deliveryattempt',
            index=models.Index(fields=['lead', 'attempt_no'], name='leads_deliv_lead_id_d4e5f6_idx'),
        ),
    ]
